package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

// handleSearch finds songs via yt-dlp and returns the results.
func handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		http.Error(w, "query required", http.StatusBadRequest)
		return
	}
	if req.Limit < 1 || req.Limit > 50 {
		req.Limit = 10
	}

	results, err := searchYouTube(req.Query, req.Limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	logSearch(req.Query, len(results))
	writeJSON(w, SearchResponse{Results: results, Count: len(results)})
}

// handleStream streams audio for instant playback.
func handleStream(w http.ResponseWriter, r *http.Request) {
	streamAudio(w, r)
}

// handleDownload starts an async single-song download.
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
	startDownload(req)
	logActivity("download", "", "", req.Artist, "", req.URL, 0, false)
	writeJSON(w, map[string]string{"status": "started", "url": req.URL})
}

// handleLibrary returns the download folder grouped by artist and album.
func handleLibrary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, buildLibrary())
}

// handleAllSongs returns every downloaded song with pagination.
func handleAllSongs(w http.ResponseWriter, r *http.Request) {
	all := allSongs()
	page := atoiDefault(r.URL.Query().Get("page"), 1)
	limit := atoiDefault(r.URL.Query().Get("limit"), 20)

	resp := all
	resp.Page = page
	resp.Limit = limit
	resp.Songs = paginateSongs(all.Songs, page, limit)
	writeJSON(w, resp)
}

// handleFile serves a downloaded audio file.
func handleFile(w http.ResponseWriter, r *http.Request) {
	serveAudioFile(w, r)
}

// handleActivity POSTs a new activity event or GETs recent activity.
func handleActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req ActivityReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		logActivity(req.Action, "", req.Track, req.Artist, req.Query, req.URL, 0, req.Action == "play")
		writeJSON(w, map[string]string{"status": "ok"})
		return
	}
	limit := atoiDefault(r.URL.Query().Get("limit"), 50)
	if limit < 1 {
		limit = 1
	}
	entries := getRecentActivity(limit)
	if entries == nil {
		entries = []ActivityEntry{}
	}
	writeJSON(w, map[string]interface{}{"activity": entries})
}

// handleSuggestions returns AI (or default) search suggestions from history.
func handleSuggestions(w http.ResponseWriter, r *http.Request) {
	topArtists := getTopArtists(10)
	recentSearches := getRecentSearches(10)
	recentActivity := getRecentActivity(20)

	if len(topArtists) == 0 && len(recentSearches) == 0 {
		writeJSON(w, SuggestionResponse{
			Suggestions: []SuggestionItem{
				{Query: "Atif Aslam best songs", Reason: "Popular artist", Type: "discovery"},
				{Query: "Pakistani classical music", Reason: "Trending genre", Type: "trending"},
				{Query: "Arabic nasheed", Reason: "Relaxing", Type: "mood"},
				{Query: "Bilal Saeed songs", Reason: "Popular artist", Type: "discovery"},
			},
			Source: "defaults",
		})
		return
	}

	suggestions := getAISuggestions(topArtists, recentSearches, recentActivity)
	writeJSON(w, SuggestionResponse{Suggestions: suggestions, Source: "ai"})
}

// handlePlaylistSuggest returns an AI playlist idea from a one-line mood.
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
	if strings.TrimSpace(req.Description) == "" {
		http.Error(w, "description required", http.StatusBadRequest)
		return
	}
	writeJSON(w, generatePlaylistSuggestion(req.Description))
}

// handleCleanTitle cleans up a messy song title using the AI.
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
	writeJSON(w, cleanTitle(req.Title))
}

// handleLibraryPlaylist builds an AI playlist from the user's downloaded
// songs. POST with {"description": "...", "track_count": N}.
func handleLibraryPlaylist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Description string `json:"description"`
		TrackCount  int    `json:"track_count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Description) == "" {
		http.Error(w, "description required", http.StatusBadRequest)
		return
	}

	playlist, err := buildLibraryPlaylist(req.Description, req.TrackCount)
	if err != nil {
		writeJSON(w, map[string]interface{}{"error": libraryPlaylistError})
		return
	}
	writeJSON(w, playlist)
}

// handleDownloadPath returns the current download directory.
func handleDownloadPath(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"path": dlRoot})
}
