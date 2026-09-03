package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// batchJob holds the single currently-running batch download (one at a time).
var batchJob = struct {
	sync.RWMutex
	job *BatchJob
}{job: nil}

// handleBatchParse uses the AI to structure raw, messy pasted text into a
// clean list of {title, artist, url} entries.
func handleBatchParse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		writeJSON(w, map[string]interface{}{"entries": []BatchEntry{}, "source": "empty"})
		return
	}

	prompt := fmt.Sprintf(`You convert raw, messy pasted music text into a clean JSON list of songs. The text may contain song titles, artists, YouTube/Music URLs, markdown links, and separators like " — " or " - " or " by " in any order.

The STRONG convention is: "TITLE — ARTIST" (title first, artist after the em-dash/hyphen). A URL in brackets [TITLE](url) refers to that TITLE.

INPUT:
%s

Return ONLY a valid JSON array, no explanation, no markdown fences. Each element: {"title": "...", "artist": "...", "url": "...", "album": ""}

RULES:
- "title" = the song name (first part before the — separator)
- "artist" = the performer/artist name (second part after the — separator)
- "url" = the URL if present, else empty string ""
- Clean titles: remove "Official Video", "Lyrics", "HD", "Visualizer" but KEEP useful qualifiers like "(Live)", "(Enstrumantal)", "(Slow Version)"
- If a line has only a song name with no artist, set artist "".
- Skip lines with no usable song info.
- Preserve every distinct song; do not merge.`, req.Text)

	resp, err := AskLLM("", prompt, 0.1, true, 8192)
	if err != nil {
		writeJSON(w, map[string]interface{}{"entries": []BatchEntry{}, "source": "error", "error": err.Error()})
		return
	}

	entries := parseBatchJSON(resp)
	if len(entries) == 0 {
		// Fall back to a deterministic local parse so the user never sees a
		// silent empty result when the model truncates or mangles output.
		entries = parseBatchFallback(req.Text)
	}
	if len(entries) == 0 {
		writeJSON(w, map[string]interface{}{
			"entries": []BatchEntry{}, "source": "error",
			"error": "The AI returned no usable songs. It may still be loading (first cloud call can be slow) — try again, or check the AI status.",
		})
		return
	}
	writeJSON(w, map[string]interface{}{"entries": entries, "source": "ollama"})
}

// handleBatchRun starts a background download job for a parsed list.
func handleBatchRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Entries      []BatchEntry `json:"entries"`
		Organization string       `json:"organization"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.Organization == "" {
		req.Organization = "artist_album"
	}

	batchJob.Lock()
	if batchJob.job != nil && batchJob.job.Status == "running" {
		batchJob.Unlock()
		http.Error(w, "A batch download is already running", http.StatusConflict)
		return
	}
	job := &BatchJob{
		ID:      fmt.Sprintf("batch-%d", time.Now().Unix()),
		Entries: req.Entries,
		Status:  "queued",
		Total:   len(req.Entries),
		Started: time.Now(),
	}
	batchJob.job = job
	jobCopy := *job
	batchJob.Unlock()

	go runBatchJob(req.Organization)

	writeJSON(w, jobCopy)
}

// runBatchJob processes each entry: resolve missing URLs, download, report.
func runBatchJob(org string) {
	batchJob.Lock()
	j := batchJob.job
	j.Status = "running"
	batchJob.Unlock()

	for i := range j.Entries {
		e := j.Entries[i]
		if j.Status != "running" {
			break
		}

		batchJob.Lock()
		j.Current = e.Title
		j.Done = i
		batchJob.Unlock()

		targetURL := e.URL
		if targetURL == "" {
			targetURL = resolveYouTubeURL(e.Title, e.Artist)
		}
		if targetURL == "" {
			batchJob.Lock()
			j.Failed++
			batchJob.Unlock()
			logActivity("download_error", "", "", e.Artist, "", "", 0, false)
			continue
		}

		tmpl := outputTemplate(org, e.Artist, e.Album)
		cmd := exec.Command(ytdlp,
			"-x", "--audio-format", "mp3", "--audio-quality", audioQuality,
			"--embed-thumbnail", "--embed-metadata", "--restrict-filenames",
			"--no-playlist", "--no-warnings",
			"--extractor-args", "youtube:player_client=web_embedded",
			"-o", tmpl, targetURL,
		)
		if _, err := cmd.CombinedOutput(); err != nil {
			log.Printf("batch download error %s: %v", targetURL, err)
			batchJob.Lock()
			j.Failed++
			batchJob.Unlock()
			logActivity("download_error", "", "", e.Artist, "", targetURL, 0, false)
		} else {
			log.Printf("batch downloaded: %s (%s)", e.Title, targetURL)
			batchJob.Lock()
			j.Done++
			batchJob.Unlock()
			logActivity("download", "", e.Title, e.Artist, "", targetURL, 0, false)
		}
	}

	batchJob.Lock()
	j.Status = "done"
	j.Done = j.Total - j.Failed
	j.Finished = time.Now()
	batchJob.Unlock()
	syncDBWithDisk()
}

// handleBatchStatus returns the current batch job progress.
func handleBatchStatus(w http.ResponseWriter, r *http.Request) {
	batchJob.RLock()
	job := batchJob.job
	batchJob.RUnlock()
	if job == nil {
		writeJSON(w, map[string]interface{}{"status": "idle"})
		return
	}

	type status struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		Total    int    `json:"total"`
		Done     int    `json:"done"`
		Failed   int    `json:"failed"`
		Skipped  int    `json:"skipped"`
		Current  string `json:"current"`
		Started  string `json:"started"`
		Finished string `json:"finished,omitempty"`
	}
	s := status{
		ID: job.ID, Status: job.Status, Total: job.Total, Done: job.Done,
		Failed: job.Failed, Skipped: job.Skipped, Current: job.Current,
	}
	if !job.Started.IsZero() {
		s.Started = job.Started.Format(time.RFC3339)
	}
	if !job.Finished.IsZero() {
		s.Finished = job.Finished.Format(time.RFC3339)
	}
	writeJSON(w, s)
}

// parseBatchFallback deterministically splits raw text into title/artist/url
// entries without an LLM, used when the model output can't be structured.
func parseBatchFallback(raw string) []BatchEntry {
	var entries []BatchEntry
	seen := map[string]bool{}
	add := func(title, artist, url string) {
		title = strings.TrimSpace(title)
		artist = strings.TrimSpace(artist)
		if title == "" {
			return
		}
		key := strings.ToLower(title + "|" + artist)
		if seen[key] {
			return
		}
		seen[key] = true
		entries = append(entries, BatchEntry{Title: title, Artist: artist, URL: url})
	}
	urlRe := regexp.MustCompile(`https?://[^\s\)\]]+`)
	linkRe := regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	splitRe := regexp.MustCompile(`\s*(?:—|–|-|\||~|vs\.?|by)\s+`)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		url := ""
		if m := urlRe.FindString(line); m != "" {
			url = m
		}
		line = linkRe.ReplaceAllString(line, "$1")
		line = urlRe.ReplaceAllString(line, "")
		line = strings.Trim(line, " \t-–—·•")
		if line == "" {
			continue
		}
		parts := splitRe.Split(line, 2)
		if len(parts) == 2 {
			add(parts[0], parts[1], url)
		} else {
			add(parts[0], "", url)
		}
	}
	return entries
}

// parseBatchJSON extracts a JSON array from the LLM response, tolerating
// surrounding markdown/text, and deduplicates.
func parseBatchJSON(raw string) []BatchEntry {
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start == -1 || end == -1 || end <= start {
		return []BatchEntry{}
	}
	var entries []BatchEntry
	if err := json.Unmarshal([]byte(raw[start:end+1]), &entries); err != nil {
		return []BatchEntry{}
	}
	seen := map[string]bool{}
	cleaned := []BatchEntry{}
	for _, e := range entries {
		key := e.URL
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(e.Title + "|" + e.Artist))
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		e.Title = strings.TrimSpace(e.Title)
		e.Artist = strings.TrimSpace(e.Artist)
		cleaned = append(cleaned, e)
	}
	return cleaned
}
