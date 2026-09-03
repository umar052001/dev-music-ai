package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ─── Config ─────────────────────────────────────────────────────────────────

const (
	addr         = ":8000"
	downloadRoot = "./downloads"
	frontendDir  = "./frontend/dist"
	ytdlp        = "yt-dlp"
	audioQuality = "320K"
)

var musicExtSet = map[string]bool{
	".mp3": true, ".m4a": true, ".flac": true,
	".wav": true, ".ogg": true, ".opus": true, ".aac": true,
}

// ─── Models ─────────────────────────────────────────────────────────────────

type SearchResult struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	CleanTitle string `json:"clean_title,omitempty"`
	Uploader string  `json:"uploader"`
	Duration float64 `json:"duration"`
	URL      string  `json:"url"`
}

type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Count   int            `json:"count"`
}

type TrackFile struct {
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Modified float64 `json:"modified"`
	Artist   string  `json:"artist"`
	Album    string  `json:"album"`
	Path     string  `json:"path"`
}

type AlbumNode struct {
	Name   string      `json:"name"`
	Tracks []TrackFile `json:"tracks"`
}

type ArtistNode struct {
	Name   string      `json:"name"`
	Albums []AlbumNode `json:"albums"`
}

type LibraryResponse struct {
	Root    string       `json:"root"`
	Artists []ArtistNode `json:"artists"`
}

type AllSongsResponse struct {
	Songs []TrackFile `json:"songs"`
	Count int         `json:"count"`
	Total int64       `json:"total_size"`
	Page  int         `json:"page"`
	Limit int         `json:"limit"`
}

type DownloadReq struct {
	URL          string `json:"url"`
	Artist       string `json:"artist"`
	Album        string `json:"album"`
	Organization string `json:"organization"`
}

type ActivityReq struct {
	Action string `json:"action"` // play, search, download, skip
	Track  string `json:"track"`
	Artist string `json:"artist"`
	Query  string `json:"query"`
	URL    string `json:"url"`
}

type SuggestionResponse struct {
	Suggestions []SuggestionItem `json:"suggestions"`
	Source      string           `json:"source"`
}

type SuggestionItem struct {
	Query  string `json:"query"`
	Reason string `json:"reason"`
	Type   string `json:"type"` // similar, trending, mood, discovery
}

type PlaylistSuggestion struct {
	Name   string   `json:"name"`
	Query  string   `json:"query"`
	Tracks int      `json:"track_count"`
	Mood   string   `json:"mood"`
}

type TitleCleanResponse struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Album  string `json:"album"`
}

type BatchEntry struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
	URL    string `json:"url"`
	Album  string `json:"album"`
	Skip   bool   `json:"skip,omitempty"`
}

type BatchJob struct {
	ID       string       `json:"id"`
	Entries  []BatchEntry `json:"entries,omitempty"`
	Status   string       `json:"status"` // queued, running, done, error
	Total    int          `json:"total"`
	Done     int          `json:"done"`
	Failed   int          `json:"failed"`
	Skipped  int          `json:"skipped"`
	Current  string       `json:"current"`
	Started  time.Time    `json:"started"`
	Finished time.Time    `json:"finished,omitempty"`
}

// ─── Globals ────────────────────────────────────────────────────────────────

var (
	dlRoot string
	db     *sql.DB
	dlStatus = struct {
		sync.RWMutex
		tasks map[string]string
	}{tasks: make(map[string]string)}
	batchJob = struct {
		sync.RWMutex
		job *BatchJob
	}{job: nil}
)

// ─── Database ───────────────────────────────────────────────────────────────

func initDB(path string) {
	var err error
	db, err = sql.Open("sqlite3", path+"?_journal_mode=WAL")
	if err != nil {
		log.Fatal(err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS tracks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		artist TEXT DEFAULT '',
		album TEXT DEFAULT '',
		path TEXT UNIQUE NOT NULL,
		youtube_id TEXT DEFAULT '',
		youtube_url TEXT DEFAULT '',
		genre TEXT DEFAULT '',
		added_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS activity (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		action TEXT NOT NULL,
		track_path TEXT DEFAULT '',
		track_name TEXT DEFAULT '',
		artist TEXT DEFAULT '',
		query TEXT DEFAULT '',
		youtube_url TEXT DEFAULT '',
		duration_ms INTEGER DEFAULT 0,
		completed BOOLEAN DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS search_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		query TEXT NOT NULL,
		result_count INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS artist_prefs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		artist TEXT UNIQUE NOT NULL,
		play_count INTEGER DEFAULT 0,
		last_played DATETIME,
		affinity REAL DEFAULT 0.0
	);

	CREATE INDEX IF NOT EXISTS idx_activity_created ON activity(created_at);
	CREATE INDEX IF NOT EXISTS idx_activity_artist ON activity(artist);
	CREATE INDEX IF NOT EXISTS idx_activity_action ON activity(action);
	CREATE INDEX IF NOT EXISTS idx_tracks_artist ON tracks(artist);
	`
	_, err = db.Exec(schema)
	if err != nil {
		log.Fatal(err)
	}
}

func logActivity(action, trackPath, trackName, artist, query, youtubeURL string, durationMs int, completed bool) {
	db.Exec(`INSERT INTO activity (action, track_path, track_name, artist, query, youtube_url, duration_ms, completed) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		action, trackPath, trackName, artist, query, youtubeURL, durationMs, completed)

	if action == "play" && artist != "" {
		db.Exec(`INSERT INTO artist_prefs (artist, play_count, last_played, affinity)
			VALUES (?, 1, CURRENT_TIMESTAMP, 1.0)
			ON CONFLICT(artist) DO UPDATE SET
				play_count = play_count + 1,
				last_played = CURRENT_TIMESTAMP,
				affinity = affinity + 0.1`,
			artist)
	}
}

func logSearch(query string, resultCount int) {
	db.Exec(`INSERT INTO search_history (query, result_count) VALUES (?, ?)`, query, resultCount)
}

func getTopArtists(limit int) []string {
	rows, err := db.Query(`SELECT artist FROM artist_prefs WHERE play_count > 0 ORDER BY affinity DESC LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var artists []string
	for rows.Next() {
		var a string
		if rows.Scan(&a) == nil && a != "" {
			artists = append(artists, a)
		}
	}
	return artists
}

func getRecentActivity(limit int) []ActivityEntry {
	rows, err := db.Query(`SELECT action, track_name, artist, query, created_at FROM activity ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var entries []ActivityEntry
	for rows.Next() {
		var e ActivityEntry
		if rows.Scan(&e.Action, &e.Track, &e.Artist, &e.Query, &e.CreatedAt) == nil {
			entries = append(entries, e)
		}
	}
	return entries
}

func getRecentSearches(limit int) []string {
	rows, err := db.Query(`SELECT DISTINCT query FROM search_history ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var queries []string
	for rows.Next() {
		var q string
		if rows.Scan(&q) == nil {
			queries = append(queries, q)
		}
	}
	return queries
}

type ActivityEntry struct {
	Action    string `json:"action"`
	Track     string `json:"track"`
	Artist    string `json:"artist"`
	Query     string `json:"query"`
	CreatedAt string `json:"created_at"`
}

func getRecentActivityJSON(limit int) []ActivityEntry {
	return getRecentActivity(limit)
}

func cleanTitle(rawTitle string) TitleCleanResponse {
	prompt := fmt.Sprintf(`Clean up this YouTube video title into proper song metadata. Return ONLY valid JSON, no explanation.

Title: "%s"

Return format: {"title": "clean song title", "artist": "artist name", "album": "album name or empty string"}

Rules:
- Remove "Official Video", "Official Audio", "HD", "4K", "Lyrics", channel names, pipe characters, dashes separating artist from title
- Extract the actual song title and artist name
- If the title already looks clean, just return it as-is
- Keep it simple and accurate`, rawTitle)

	resp, err := AskLLM("", prompt, 0.2, true)
	if err != nil {
		// Fallback: simple cleanup
		cleaned := rawTitle
		cleaned = regexp.MustCompile(`\s*[\|–\-]\s*(Official|Music|Video|Audio|Lyrics|HD|4K|Visualizer).*`).ReplaceAllString(cleaned, "")
		cleaned = regexp.MustCompile(`\s*[\|–\-]\s*[A-Z][a-z]+ Music.*`).ReplaceAllString(cleaned, "")
		cleaned = strings.TrimSpace(cleaned)
		return TitleCleanResponse{Title: cleaned, Artist: "", Album: ""}
	}

	// Parse JSON response
	resp = strings.TrimSpace(resp)
	if idx := strings.Index(resp, "{"); idx >= 0 {
		if endIdx := strings.LastIndex(resp, "}"); endIdx > idx {
			resp = resp[idx : endIdx+1]
		}
	}
	var result TitleCleanResponse
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return TitleCleanResponse{Title: rawTitle}
	}
	if result.Title == "" {
		result.Title = rawTitle
	}
	return result
}

func getAISuggestions(topArtists []string, recentSearches []string, recentActivity []ActivityEntry) []SuggestionItem {
	activityText := ""
	for _, a := range recentActivity[:min(15, len(recentActivity))] {
		activityText += fmt.Sprintf("- %s: %s by %s\n", a.Action, a.Track, a.Artist)
	}
	searchText := strings.Join(recentSearches[:min(10, len(recentSearches))], ", ")
	artistText := strings.Join(topArtists[:min(10, len(topArtists))], ", ")

	prompt := fmt.Sprintf(`You are a music recommendation engine. Based on the user's listening history, suggest 8 music search queries they might enjoy.

Top artists: %s
Recent searches: %s
Recent activity:
%s

Return ONLY a JSON array of objects with "query" (search query), "reason" (brief reason), and "type" (one of: similar, discovery, mood, trending).

Example:
[{"query":"Atif Aslam sad songs","reason":"You listen to Atif Aslam often","type":"similar"}]

Rules:
- Mix types: some similar to their taste, some discovery/new artists
- Keep queries specific enough to find good results on YouTube
- Consider mood from the activity (if they play sad songs, suggest similar mood)
- Return ONLY the JSON array, no explanation`, artistText, searchText, activityText)

	resp, err := AskLLM("", prompt, 0.7, false)
	if err != nil {
		return getFallbackSuggestions(topArtists)
	}

	resp = strings.TrimSpace(resp)
	if idx := strings.Index(resp, "["); idx >= 0 {
		resp = resp[idx:]
	}
	if endIdx := strings.LastIndex(resp, "]"); endIdx >= 0 {
		resp = resp[:endIdx+1]
	}

	var items []SuggestionItem
	if err := json.Unmarshal([]byte(resp), &items); err != nil {
		return getFallbackSuggestions(topArtists)
	}
	return items
}

func getFallbackSuggestions(topArtists []string) []SuggestionItem {
	var items []SuggestionItem
	for _, a := range topArtists[:min(5, len(topArtists))] {
		items = append(items, SuggestionItem{
			Query:  a + " best songs",
			Reason: "Based on your listening",
			Type:   "similar",
		})
	}
	if len(items) == 0 {
		items = append(items,
			SuggestionItem{Query: "Pakistani classical music", Reason: "Popular genre", Type: "discovery"},
			SuggestionItem{Query: "Arabic nasheed", Reason: "Trending", Type: "trending"},
		)
	}
	return items
}

func generatePlaylistSuggestion(description string) PlaylistSuggestion {
	prompt := fmt.Sprintf(`Generate a music playlist suggestion. Return ONLY valid JSON.

User request: "%s"

Return format: {"name": "playlist name", "query": "YouTube search query to find these songs", "track_count": 15, "mood": "mood description"}

Make the query specific enough to find good results. Include genre, era, or mood keywords.`, description)

	resp, err := AskLLM("", prompt, 0.6, false)
	if err != nil {
		return PlaylistSuggestion{
			Name:   description,
			Query:  description,
			Tracks: 10,
			Mood:   "varied",
		}
	}

	resp = strings.TrimSpace(resp)
	if idx := strings.Index(resp, "{"); idx >= 0 {
		if endIdx := strings.LastIndex(resp, "}"); endIdx > idx {
			resp = resp[idx : endIdx+1]
		}
	}

	var result PlaylistSuggestion
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return PlaylistSuggestion{Name: description, Query: description, Tracks: 10, Mood: "varied"}
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func safeName(s string) string {
	re := regexp.MustCompile(`[\\/:*?"<>|\n\r\t]`)
	s = re.ReplaceAllString(s, "_")
	s = strings.Trim(s, ". ")
	if s == "" {
		return "Unknown"
	}
	return s
}

func outputTemplate(org, artist, album string) string {
	root := dlRoot
	switch org {
	case "artist_album":
		if artist != "" && album != "" {
			return filepath.Join(root, safeName(artist), safeName(album), "%(track_number|0)02d - %(title)s.%(ext)s")
		}
		if artist != "" {
			return filepath.Join(root, safeName(artist), "%(title)s.%(ext)s")
		}
		return filepath.Join(root, "%(uploader|Unknown)s", "%(album|Singles)s", "%(track_number|0)02d - %(title)s.%(ext)s")
	case "artist_only":
		if artist != "" {
			return filepath.Join(root, safeName(artist), "%(title)s.%(ext)s")
		}
		return filepath.Join(root, "%(uploader|Unknown)s", "%(title)s.%(ext)s")
	default:
		return filepath.Join(root, "%(playlist|Unknown)s", "%(playlist_index)03d - %(title)s.%(ext)s")
	}
}

func collectTracks(dirPath, artist, album string) []TrackFile {
	var tracks []TrackFile
	entries, _ := os.ReadDir(dirPath)
	for _, f := range entries {
		if f.IsDir() || !musicExtSet[filepath.Ext(f.Name())] {
			continue
		}
		info, _ := f.Info()
		if info == nil {
			continue
		}
		relPath, _ := filepath.Rel(dlRoot, filepath.Join(dirPath, f.Name()))
		tracks = append(tracks, TrackFile{
			Name:     f.Name(),
			Size:     info.Size(),
			Modified: float64(info.ModTime().Unix()),
			Artist:   artist,
			Album:    album,
			Path:     filepath.ToSlash(relPath),
		})
	}
	return tracks
}

func syncDBWithDisk() {
	entries, err := os.ReadDir(dlRoot)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		artistName := entry.Name()
		artistPath := filepath.Join(dlRoot, artistName)

		tracks := collectTracks(artistPath, artistName, "Singles")
		for _, t := range tracks {
			db.Exec(`INSERT OR IGNORE INTO tracks (name, artist, album, path) VALUES (?, ?, ?, ?)`,
				t.Name, t.Artist, t.Album, t.Path)
		}

		subEntries, _ := os.ReadDir(artistPath)
		for _, sub := range subEntries {
			if !sub.IsDir() {
				continue
			}
			albumTracks := collectTracks(filepath.Join(artistPath, sub.Name()), artistName, sub.Name())
			for _, t := range albumTracks {
				db.Exec(`INSERT OR IGNORE INTO tracks (name, artist, album, path) VALUES (?, ?, ?, ?)`,
					t.Name, t.Artist, t.Album, t.Path)
			}
		}
	}
}

// ─── Handlers ───────────────────────────────────────────────────────────────

func handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Query      string `json:"query"`
		SearchType string `json:"search_type"`
		Limit      int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.Query == "" {
		http.Error(w, "query required", http.StatusBadRequest)
		return
	}
	if req.Limit < 1 || req.Limit > 50 {
		req.Limit = 10
	}

	cmd := exec.Command(ytdlp,
		"--flat-playlist", "--dump-json", "--no-warnings",
		fmt.Sprintf("ytsearch%d:%s", req.Limit, req.Query),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("yt-dlp search error: %v", err)
		http.Error(w, string(out), http.StatusInternalServerError)
		return
	}

	var results []SearchResult
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		id := getString(obj, "id")
		title := getString(obj, "title")
		url := getString(obj, "webpage_url")
		if url == "" && id != "" {
			url = "https://www.youtube.com/watch?v=" + id
		}
		results = append(results, SearchResult{
			ID:       id,
			Title:    title,
			Uploader: getString(obj, "uploader"),
			Duration: getFloat(obj, "duration"),
			URL:      url,
		})
	}

	logSearch(req.Query, len(results))
	json.NewEncoder(w).Encode(SearchResponse{Results: results, Count: len(results)})
}

func handleStream(w http.ResponseWriter, r *http.Request) {
	videoURL := r.URL.Query().Get("url")
	if videoURL == "" {
		http.Error(w, "url param required", http.StatusBadRequest)
		return
	}

	// Determine the actual container/extension of the best audio so we can
	// set a correct Content-Type for the browser. Default to webm (opus).
	ext := "webm"
	if out, err := exec.Command(ytdlp,
		"--no-warnings", "--no-playlist", "--print", "ext",
		"--extractor-args", "youtube:player_client=web_embedded",
		"-f", "bestaudio/best", videoURL,
	).Output(); err == nil {
		if e := strings.TrimSpace(string(out)); e != "" {
			ext = e
		}
	}

	mime := "audio/webm"
	switch ext {
	case "m4a", "mp4":
		mime = "audio/mp4"
	case "mp3":
		mime = "audio/mpeg"
	case "ogg", "opus":
		mime = "audio/ogg"
	}

	// Progressive chunked streaming with no buffering delays
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Accept-Ranges", "none")

	// Force re-encode to mp3 when requested (universally playable), otherwise
	// stream the native audio for instant start.
	var cmd *exec.Cmd
	if r.URL.Query().Get("format") == "mp3" {
		cmd = exec.Command(ytdlp,
			"-x", "--audio-format", "mp3", "--audio-quality", audioQuality,
			"--no-playlist", "--no-warnings", "-o", "-",
			"--extractor-args", "youtube:player_client=web_embedded",
			"-f", "bestaudio/best", videoURL,
		)
		w.Header().Set("Content-Type", "audio/mpeg")
	} else {
		// Stream native webm/opus directly — starts near-instantly, no transcode
		cmd = exec.Command(ytdlp,
			"--no-playlist", "--no-warnings", "-o", "-",
			"--extractor-args", "youtube:player_client=web_embedded",
			"-f", "bestaudio/best", videoURL,
		)
	}

	// Wrap stdout so we flush after each write for progressive streaming
	cmd.Stdout = &flushWriter{w: w, f: flusher}
	cmd.Stderr = os.Stderr
	log.Printf("streaming (%s): %s", mime, videoURL)
	if err := cmd.Run(); err != nil {
		log.Printf("stream error: %v", err)
	}
}

type flushWriter struct {
	w http.ResponseWriter
	f http.Flusher
}

func (fw *flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if fw.f != nil {
		fw.f.Flush()
	}
	return n, err
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req DownloadReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		http.Error(w, "url required", http.StatusBadRequest)
		return
	}
	if req.Organization == "" {
		req.Organization = "artist_album"
	}
	tmpl := outputTemplate(req.Organization, req.Artist, req.Album)

	dlStatus.Lock()
	dlStatus.tasks[req.URL] = "downloading"
	dlStatus.Unlock()

	go func() {
		defer func() {
			dlStatus.Lock()
			dlStatus.tasks[req.URL] = "done"
			dlStatus.Unlock()
			syncDBWithDisk()
		}()
		cmd := exec.Command(ytdlp,
			"-x", "--audio-format", "mp3", "--audio-quality", audioQuality,
			"--embed-thumbnail", "--embed-metadata", "--restrict-filenames",
			"--no-playlist", "--no-warnings", "-o", tmpl, req.URL,
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("download error %s: %v\n%s", req.URL, err, string(out))
			dlStatus.Lock()
			dlStatus.tasks[req.URL] = "error"
			dlStatus.Unlock()
		} else {
			log.Printf("downloaded: %s", req.URL)
		}
	}()
	logActivity("download", "", "", req.Artist, "", req.URL, 0, false)
	json.NewEncoder(w).Encode(map[string]string{"status": "started", "url": req.URL})
}

func handleLibrary(w http.ResponseWriter, r *http.Request) {
	lib := LibraryResponse{Root: dlRoot, Artists: []ArtistNode{}}
	entries, err := os.ReadDir(dlRoot)
	if err != nil {
		json.NewEncoder(w).Encode(lib)
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		artistPath := filepath.Join(dlRoot, entry.Name())
		an := ArtistNode{Name: entry.Name()}
		singles := collectTracks(artistPath, entry.Name(), "Singles")
		if len(singles) > 0 {
			sort.Slice(singles, func(i, j int) bool { return singles[i].Name < singles[j].Name })
			an.Albums = append(an.Albums, AlbumNode{Name: "Singles", Tracks: singles})
		}
		subEntries, _ := os.ReadDir(artistPath)
		for _, sub := range subEntries {
			if !sub.IsDir() {
				continue
			}
			tracks := collectTracks(filepath.Join(artistPath, sub.Name()), entry.Name(), sub.Name())
			if len(tracks) > 0 {
				sort.Slice(tracks, func(i, j int) bool { return tracks[i].Name < tracks[j].Name })
				an.Albums = append(an.Albums, AlbumNode{Name: sub.Name(), Tracks: tracks})
			}
		}
		if len(an.Albums) > 0 {
			lib.Artists = append(lib.Artists, an)
		}
	}
	json.NewEncoder(w).Encode(lib)
}

func handleAllSongs(w http.ResponseWriter, r *http.Request) {
	resp := AllSongsResponse{Songs: []TrackFile{}}
	entries, err := os.ReadDir(dlRoot)
	if err != nil {
		json.NewEncoder(w).Encode(resp)
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		artistName := entry.Name()
		tracks := collectTracks(filepath.Join(dlRoot, artistName), artistName, "Singles")
		resp.Songs = append(resp.Songs, tracks...)
		subEntries, _ := os.ReadDir(filepath.Join(dlRoot, artistName))
		for _, sub := range subEntries {
			if !sub.IsDir() {
				continue
			}
			albumTracks := collectTracks(filepath.Join(dlRoot, artistName, sub.Name()), artistName, sub.Name())
			resp.Songs = append(resp.Songs, albumTracks...)
		}
	}
	sort.Slice(resp.Songs, func(i, j int) bool { return resp.Songs[i].Name < resp.Songs[j].Name })
	var total int64
	for _, s := range resp.Songs {
		total += s.Size
	}
	resp.Count = len(resp.Songs)
	resp.Total = total

	// Pagination
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 1 { page = 1 }
	if limit < 1 { limit = 20 }
	if limit > 100 { limit = 100 }
	start := (page - 1) * limit
	if start > len(resp.Songs) {
		resp.Songs = []TrackFile{}
	} else {
		end := start + limit
		if end > len(resp.Songs) { end = len(resp.Songs) }
		resp.Songs = resp.Songs[start:end]
	}
	resp.Page = page
	resp.Limit = limit

	json.NewEncoder(w).Encode(resp)
}

func handleFile(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/file/")
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}
	full := filepath.Join(dlRoot, path)
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	contentTypes := map[string]string{
		".mp3": "audio/mpeg", ".m4a": "audio/mp4", ".flac": "audio/flac",
		".wav": "audio/wav", ".ogg": "audio/ogg", ".opus": "audio/opus", ".aac": "audio/aac",
	}
	ext := strings.ToLower(filepath.Ext(full))
	if ct, ok := contentTypes[ext]; ok {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeFile(w, r, full)
}

func handleActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req ActivityReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		logActivity(req.Action, "", req.Track, req.Artist, req.Query, req.URL, 0, req.Action == "play")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}
	// GET: return recent activity
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}
	entries := getRecentActivityJSON(limit)
	if entries == nil {
		entries = []ActivityEntry{}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"activity": entries})
}

func handleSuggestions(w http.ResponseWriter, r *http.Request) {
	topArtists := getTopArtists(10)
	recentSearches := getRecentSearches(10)
	recentActivity := getRecentActivity(20)

	if len(topArtists) == 0 && len(recentSearches) == 0 {
		// No history yet, return defaults
		json.NewEncoder(w).Encode(SuggestionResponse{
			Suggestions: []SuggestionItem{
				{Query: "Atif Aslam best songs", Reason: "Popular artist", Type: "discovery"},
				{Query: "Pakistani classical music", Reason: "Trending genre", Type: "trending"},
				{Query: "Arabic nasheed nasheed", Reason: "Relaxing", Type: "mood"},
				{Query: "Bilal Saeed songs", Reason: "Popular artist", Type: "discovery"},
			},
			Source: "defaults",
		})
		return
	}

	suggestions := getAISuggestions(topArtists, recentSearches, recentActivity)
	json.NewEncoder(w).Encode(SuggestionResponse{Suggestions: suggestions, Source: "ai"})
}

func handlePlaylistSuggest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.Description == "" {
		http.Error(w, "description required", http.StatusBadRequest)
		return
	}
	playlist := generatePlaylistSuggestion(req.Description)
	json.NewEncoder(w).Encode(playlist)
}

func handleCleanTitle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	result := cleanTitle(req.Title)
	json.NewEncoder(w).Encode(result)
}

// handleBatchParse uses Ollama to structure raw, messy pasted text into a
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
		json.NewEncoder(w).Encode(map[string]interface{}{"entries": []BatchEntry{}, "source": "empty"})
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
		json.NewEncoder(w).Encode(map[string]interface{}{"entries": []BatchEntry{}, "source": "error", "error": err.Error()})
		return
	}

	entries := parseBatchJSON(resp)
	if len(entries) == 0 {
		// Fall back to a deterministic local parse so the user never sees a
		// silent empty result when the model truncates or mangles output.
		entries = parseBatchFallback(req.Text)
	}
	if len(entries) == 0 {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"entries": []BatchEntry{}, "source": "error",
			"error": "The AI returned no usable songs. It may still be loading (first cloud call can be slow) — try again, or check the AI status.",
		})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"entries": entries, "source": "ollama"})
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
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Extract URL if present
		url := ""
		if m := urlRe.FindString(line); m != "" {
			url = m
		}
		// Strip markdown link syntax: [Title](url)
		line = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`).ReplaceAllString(line, "$1")
		line = urlRe.ReplaceAllString(line, "")
		line = strings.Trim(line, " \t-–—·•")
		if line == "" {
			continue
		}
		// Split on " — ", " - ", " | ", " by " (title / artist)
		parts := regexp.MustCompile(`\s*(?:—|–|-|\||~|vs\.?|by)\s+`).Split(line, 2)
		if len(parts) == 2 {
			add(parts[0], parts[1], url)
		} else {
			add(parts[0], "", url)
		}
	}
	return entries
}

// parseBatchJSON extracts a JSON array from the Ollama response, tolerating
// surrounding markdown/text.
func parseBatchJSON(raw string) []BatchEntry {
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start == -1 || end == -1 || end <= start {
		return []BatchEntry{}
	}
	jsonStr := raw[start : end+1]
	var entries []BatchEntry
	if err := json.Unmarshal([]byte(jsonStr), &entries); err != nil {
		return []BatchEntry{}
	}
	// Deduplicate by URL when possible
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
	id := fmt.Sprintf("batch-%d", time.Now().Unix())
	job := &BatchJob{
		ID:      id,
		Entries: req.Entries,
		Status:  "queued",
		Total:   len(req.Entries),
		Started: time.Now(),
	}
	batchJob.job = job
	jobCopy := *job
	batchJob.Unlock()

	go func() {
		batchJob.Lock()
		j := batchJob.job
		j.Status = "running"
		batchJob.Unlock()

		n := len(j.Entries)
		for i := 0; i < n; i++ {
			e := j.Entries[i]
			if j.Status != "running" { // cancelled
				break
			}

			batchJob.Lock()
			j.Current = e.Title
			j.Done = i
			batchJob.Unlock()

			// If entry has no URL, try to resolve it via yt-dlp search
			targetURL := e.URL
			if targetURL == "" {
				q := e.Title
				if e.Artist != "" {
					q = e.Title + " " + e.Artist
				}
				cmd := exec.Command(ytdlp, "--flat-playlist", "--dump-json", "--no-warnings",
					"--extractor-args", "youtube:player_client=web_embedded",
					fmt.Sprintf("ytsearch1:%s", q))
				out, err := cmd.Output()
				if err == nil {
					var obj map[string]interface{}
					if json.Unmarshal(out, &obj) == nil {
						if id := getString(obj, "id"); id != "" {
							targetURL = "https://www.youtube.com/watch?v=" + id
						}
					}
				}
			}

			if targetURL == "" {
				batchJob.Lock()
				j.Failed++
				batchJob.Unlock()
				logActivity("download_error", "", "", e.Artist, "", "", 0, false)
				continue
			}

			tmpl := outputTemplate(req.Organization, e.Artist, e.Album)
			cmd := exec.Command(ytdlp,
				"-x", "--audio-format", "mp3", "--audio-quality", audioQuality,
				"--embed-thumbnail", "--embed-metadata", "--restrict-filenames",
				"--no-playlist", "--no-warnings",
				"--extractor-args", "youtube:player_client=web_embedded",
				"-o", tmpl, targetURL,
			)
			dOut, dErr := cmd.CombinedOutput()
			if dErr != nil {
				log.Printf("batch download error %s: %v", targetURL, dErr)
				batchJob.Lock()
				j.Failed++
				batchJob.Unlock()
				logActivity("download_error", "", "", e.Artist, "", targetURL, 0, false)
			} else {
				_ = dOut
				log.Printf("batch downloaded: %s (%s)", e.Title, targetURL)
				batchJob.Lock()
				j.Done++
				batchJob.Unlock()
				logActivity("download", "", e.Title, e.Artist, "", targetURL, 0, false)
			}
		}

		batchJob.Lock()
		jx := batchJob.job
		jx.Status = "done"
		jx.Done = jx.Total - jx.Failed
		jx.Finished = time.Now()
		batchJob.Unlock()
		syncDBWithDisk()
	}()

	json.NewEncoder(w).Encode(jobCopy)
}

// handleBatchStatus returns the current batch job progress.
func handleBatchStatus(w http.ResponseWriter, r *http.Request) {
	batchJob.RLock()
	job := batchJob.job
	batchJob.RUnlock()
	if job == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "idle"})
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
		ID:      job.ID,
		Status:  job.Status,
		Total:   job.Total,
		Done:    job.Done,
		Failed:  job.Failed,
		Skipped: job.Skipped,
		Current: job.Current,
	}
	if !job.Started.IsZero() {
		s.Started = job.Started.Format(time.RFC3339)
	}
	if !job.Finished.IsZero() {
		s.Finished = job.Finished.Format(time.RFC3339)
	}
	json.NewEncoder(w).Encode(s)
}

func handleDownloadPath(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"path": dlRoot})
}

// ─── JSON helpers ───────────────────────────────────────────────────────────

func getString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func getFloat(m map[string]interface{}, key string) float64 {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	}
	return 0
}

// ─── SPA handler ────────────────────────────────────────────────────────────

func spaHandler(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(frontendDir, r.URL.Path)
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		http.ServeFile(w, r, path)
		return
	}
	http.ServeFile(w, r, filepath.Join(frontendDir, "index.html"))
}

// ─── CORS ───────────────────────────────────────────────────────────────────

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

// ─── Main ───────────────────────────────────────────────────────────────────

func main() {
	absRoot, _ := filepath.Abs(downloadRoot)
	dlRoot = absRoot
	os.MkdirAll(dlRoot, 0755)
	os.MkdirAll(frontendDir, 0755)

	initLLM()

	initDB("./devmusic.db")
	defer db.Close()
	syncDBWithDisk()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/search", corsMiddleware(handleSearch))
	mux.HandleFunc("/api/stream", corsMiddleware(handleStream))
	mux.HandleFunc("/api/download", corsMiddleware(handleDownload))
	mux.HandleFunc("/api/library", corsMiddleware(handleLibrary))
	mux.HandleFunc("/api/all-songs", corsMiddleware(handleAllSongs))
	mux.HandleFunc("/api/download-path", corsMiddleware(handleDownloadPath))
	mux.HandleFunc("/api/file/", corsMiddleware(handleFile))
	mux.HandleFunc("/api/activity", corsMiddleware(handleActivity))
	mux.HandleFunc("/api/suggestions", corsMiddleware(handleSuggestions))
	mux.HandleFunc("/api/playlist-suggest", corsMiddleware(handlePlaylistSuggest))
	mux.HandleFunc("/api/clean-title", corsMiddleware(handleCleanTitle))
	mux.HandleFunc("/api/batch/parse", corsMiddleware(handleBatchParse))
	mux.HandleFunc("/api/batch/run", corsMiddleware(handleBatchRun))
	mux.HandleFunc("/api/batch/status", corsMiddleware(handleBatchStatus))
	mux.HandleFunc("/api/llm/status", corsMiddleware(handleLLMStatus))
	mux.HandleFunc("/api/llm/config", corsMiddleware(handleLLMConfigGet))
	mux.HandleFunc("/api/llm/config-set", corsMiddleware(handleLLMConfigSet))

	mux.HandleFunc("/", spaHandler)

	log.Printf("Dev Music running on http://localhost%s", addr)
	log.Printf("Downloads: %s", dlRoot)
	log.Printf("Database: devmusic.db")
	lc := getLLMConfig()
	log.Printf("LLM: provider=%s model=%s fast=%s base=%s", lc.Provider, lc.Model, lc.FastModel, lc.APIBase)
	log.Fatal(http.ListenAndServe(addr, mux))
}
