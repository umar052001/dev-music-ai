package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// Configuration constants. Change these to tweak the server.
const (
	addr         = ":8000"
	downloadRoot = "./downloads"
	frontendDir  = "./frontend/dist"
	dbPath       = "./devmusic.db"
)

// main wires up configuration, the database, and the HTTP routes, then starts
// the server. It is intentionally thin — all logic lives in the handlers and
// support packages below.
func main() {
	// Resolve and prepare the download + frontend directories.
	absRoot, _ := filepath.Abs(downloadRoot)
	dlRoot = absRoot
	if err := os.MkdirAll(dlRoot, 0755); err != nil {
		log.Fatalf("create download dir: %v", err)
	}
	os.MkdirAll(frontendDir, 0755)

	// Load AI config, then open the database and index any on-disk music.
	initLLM()
	initDB(dbPath)
	defer closeDB()
	syncDBWithDisk()

	mux := http.NewServeMux()
	registerRoutes(mux)

	log.Printf("Dev Music running on http://localhost%s", addr)
	log.Printf("Downloads: %s", dlRoot)
	log.Printf("Database: %s", dbPath)
	lc := getLLMConfig()
	log.Printf("LLM: provider=%s model=%s fast=%s base=%s", lc.Provider, lc.Model, lc.FastModel, lc.APIBase)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// registerRoutes attaches every API endpoint and the SPA handler to mux.
func registerRoutes(mux *http.ServeMux) {
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
	mux.HandleFunc("/api/library/playlist", corsMiddleware(handleLibraryPlaylist))
	mux.HandleFunc("/api/batch/parse", corsMiddleware(handleBatchParse))
	mux.HandleFunc("/api/batch/run", corsMiddleware(handleBatchRun))
	mux.HandleFunc("/api/batch/status", corsMiddleware(handleBatchStatus))
	mux.HandleFunc("/api/llm/status", corsMiddleware(handleLLMStatus))
	mux.HandleFunc("/api/llm/config", corsMiddleware(handleLLMConfigGet))
	mux.HandleFunc("/api/llm/config-set", corsMiddleware(handleLLMConfigSet))

	mux.HandleFunc("/", spaHandler)
}
