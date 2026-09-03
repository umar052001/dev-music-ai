package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// dlManager coordinates background download jobs. Every job runs one-at-a-time
// (matching the original single-download behaviour) and records its progress
// in the persisted `downloads` table so status survives page refreshes and
// tab switches. History also survives server restarts (a worker can't resume,
// but the completed/failed records remain visible).
type dlManager struct {
	mu   sync.Mutex
	busy map[string]bool // batch_id -> running
}

var downloadMgr = &dlManager{busy: make(map[string]bool)}

// DownloadItem is one song queued for download.
type DownloadItem struct {
	URL    string `json:"url"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Album  string `json:"album"`
}

// enqueueDownloads persists the items under a batch and starts a background
// worker (if one isn't already running for the batch). It returns the batch id.
func enqueueDownloads(org string, items []DownloadItem) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("no items")
	}
	batchID := fmt.Sprintf("dl-%d", time.Now().UnixNano())

	// Persist every item immediately so nothing is lost on refresh.
	ids := make([]int64, 0, len(items))
	for _, it := range items {
		id, err := createDownload(it.URL, it.Title, it.Artist, it.Album, batchID)
		if err != nil {
			log.Printf("create download record: %v", err)
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return batchID, fmt.Errorf("failed to create download records")
	}

	// Start the worker for this batch if not already running.
	downloadMgr.mu.Lock()
	if !downloadMgr.busy[batchID] {
		downloadMgr.busy[batchID] = true
		go func() {
			runDownloadJob(batchID, org, items)
			downloadMgr.mu.Lock()
			delete(downloadMgr.busy, batchID)
			downloadMgr.mu.Unlock()
		}()
	}
	downloadMgr.mu.Unlock()

	return batchID, nil
}

// runDownloadJob processes each item sequentially, updating the persisted
// status for each and syncing the library as songs complete.
func runDownloadJob(batchID, org string, items []DownloadItem) {
	rows, _ := listDownloads(batchID, 100000)
	byURL := make(map[string]DownloadRow, len(rows))
	for _, r := range rows {
		byURL[r.URL] = r
	}

	for _, it := range items {
		row, ok := byURL[it.URL]
		if !ok {
			continue
		}
		setDownloadStatus(row.ID, "running", "", "")

		targetURL := it.URL
		if targetURL == "" {
			targetURL = resolveYouTubeURL(it.Title, it.Artist)
		}
		if targetURL == "" {
			setDownloadStatus(row.ID, "error", "could not resolve URL", "")
			logActivity("download_error", "", "", it.Artist, "", targetURL, 0, false)
			continue
		}

		tmpl := outputTemplate(org, it.Artist, it.Album)
		cmd := exec.Command(ytdlp,
			"-x", "--audio-format", "mp3", "--audio-quality", audioQuality,
			"--embed-thumbnail", "--embed-metadata", "--restrict-filenames",
			"--no-playlist", "--no-warnings",
			"--extractor-args", "youtube:player_client=web_embedded",
			"-o", tmpl, targetURL,
		)
		out, err := cmd.CombinedOutput()
		filePath := extractOutputPath(string(out))
		if err != nil {
			log.Printf("download error %s: %v\n%s", targetURL, err, string(out))
			setDownloadStatus(row.ID, "error", firstLine(err), "")
			logActivity("download_error", "", "", it.Artist, "", targetURL, 0, false)
			continue
		}
		log.Printf("downloaded: %s", targetURL)
		setDownloadStatus(row.ID, "done", "", filePath)
		logActivity("download", "", it.Title, it.Artist, "", targetURL, 0, false)
	}
	syncDBWithDisk()
}

// extractOutputPath pulls the final audio file path from yt-dlp's stdout.
// yt-dlp prints "[ExtractAudio] Destination: <path>" for the intermediate
// format; since we always request mp3 via -x --audio-format mp3, the final
// file shares that path with an .mp3 extension.
func extractOutputPath(out string) string {
	dest := ""
	for _, line := range strings.Split(out, "\n") {
		if idx := strings.Index(line, "Destination: "); idx >= 0 {
			dest = strings.TrimSpace(line[idx+len("Destination: "):])
		}
	}
	if dest == "" {
		return ""
	}
	return dest[:len(dest)-len(filepath.Ext(dest))] + ".mp3"
}

// firstLine returns the first non-empty line of an error message (the yt-dlp
// exit detail is usually on the first line of the error).
func firstLine(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if idx := strings.IndexByte(msg, '\n'); idx >= 0 {
		return msg[:idx]
	}
	if len(msg) > 200 {
		return msg[:200]
	}
	return msg
}

// handleDownloadsAdd enqueues one or more songs for download and returns a
// batch id so the client can poll status. Accepts either a single object
// {url, artist, album, organization} or {organization, items: [...]}.
func handleDownloadsAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		URL          string         `json:"url"`
		Title        string         `json:"title"`
		Artist       string         `json:"artist"`
		Album        string         `json:"album"`
		Organization string         `json:"organization"`
		Items        []DownloadItem `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	org := req.Organization
	if org == "" {
		org = "artist_album"
	}
	var items []DownloadItem
	if len(req.Items) > 0 {
		items = req.Items
	} else {
		if req.URL == "" {
			http.Error(w, "url required", http.StatusBadRequest)
			return
		}
		items = []DownloadItem{{URL: req.URL, Title: req.Title, Artist: req.Artist, Album: req.Album}}
	}
	batchID, err := enqueueDownloads(org, items)
	if err != nil {
		http.Error(w, "could not enqueue: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"batch_id": batchID, "count": len(items)})
}

// handleDownloadsStatus returns persisted status for a batch (or recent
// downloads when no batch_id is given).
func handleDownloadsStatus(w http.ResponseWriter, r *http.Request) {
	batchID := r.URL.Query().Get("batch_id")
	limit := atoiDefault(r.URL.Query().Get("limit"), 50)

	var items []DownloadRow
	if batchID != "" {
		items, _ = listDownloads(batchID, limit)
	} else {
		items, _ = listDownloads("", limit)
	}
	// listDownloads returns newest-first; reorder oldest-first for progress.
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}

	total, queued, running, done, failed := downloadSummary(batchID)
	active := queued + running
	writeJSON(w, map[string]interface{}{
		"batch_id": batchID,
		"total":    total,
		"queued":   queued,
		"running":  running,
		"done":     done,
		"failed":   failed,
		"active":   active,
		"complete": active == 0 && total > 0,
		"items":    items,
	})
}
