package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

// db is the shared SQLite handle, initialized once in main().
var db *sql.DB

// initDB opens (or creates) the SQLite database and applies the schema.
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

	CREATE TABLE IF NOT EXISTS downloads (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		batch_id TEXT DEFAULT '',
		url TEXT NOT NULL,
		title TEXT DEFAULT '',
		artist TEXT DEFAULT '',
		album TEXT DEFAULT '',
		status TEXT DEFAULT 'queued',   -- queued, running, done, error, cancelled
		error TEXT DEFAULT '',
		file_path TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_downloads_batch ON downloads(batch_id);
	CREATE INDEX IF NOT EXISTS idx_activity_created ON activity(created_at);
	CREATE INDEX IF NOT EXISTS idx_activity_artist ON activity(artist);
	CREATE INDEX IF NOT EXISTS idx_activity_action ON activity(action);
	CREATE INDEX IF NOT EXISTS idx_tracks_artist ON tracks(artist);
	`
	if _, err := db.Exec(schema); err != nil {
		log.Fatal(err)
	}
}

func closeDB() {
	if db != nil {
		db.Close()
	}
}

// logActivity records a user action and bumps artist affinity on plays.
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
	rows, err := db.Query(`SELECT id, action, track_name, artist, query, created_at FROM activity ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var entries []ActivityEntry
	for rows.Next() {
		var e ActivityEntry
		if rows.Scan(&e.ID, &e.Action, &e.Track, &e.Artist, &e.Query, &e.CreatedAt) == nil {
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

// DownloadRow is one persisted download record.
type DownloadRow struct {
	ID        int64  `json:"id"`
	BatchID   string `json:"batch_id"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	Album     string `json:"album"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	FilePath  string `json:"file_path,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// createDownload inserts a new queued download and returns its id.
func createDownload(url, title, artist, album, batchID string) (int64, error) {
	res, err := db.Exec(`INSERT INTO downloads (url, title, artist, album, batch_id) VALUES (?, ?, ?, ?, ?)`,
		url, title, artist, album, batchID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// setDownloadStatus updates status/error/file_path and the updated_at stamp.
func setDownloadStatus(id int64, status string, errorMsg string, filePath string) {
	db.Exec(`UPDATE downloads SET status=?, error=?, file_path=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		status, errorMsg, filePath, id)
}

// listDownloads returns persisted downloads for the given batch (or all recent
// when batchID is empty), newest-first, capped at limit.
func listDownloads(batchID string, limit int) ([]DownloadRow, error) {
	var rows *sql.Rows
	var err error
	if batchID != "" {
		rows, err = db.Query(`SELECT id, batch_id, url, title, artist, album, status, COALESCE(error,''), COALESCE(file_path,''), created_at, updated_at FROM downloads WHERE batch_id=? ORDER BY id DESC LIMIT ?`, batchID, limit)
	} else {
		rows, err = db.Query(`SELECT id, batch_id, url, title, artist, album, status, COALESCE(error,''), COALESCE(file_path,''), created_at, updated_at FROM downloads ORDER BY id DESC LIMIT ?`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DownloadRow
	for rows.Next() {
		var d DownloadRow
		if rows.Scan(&d.ID, &d.BatchID, &d.URL, &d.Title, &d.Artist, &d.Album, &d.Status, &d.Error, &d.FilePath, &d.CreatedAt, &d.UpdatedAt) == nil {
			out = append(out, d)
		}
	}
	return out, nil
}

// downloadSummary aggregates counts for a batch (or all downloads when empty).
func downloadSummary(batchID string) (total, queued, running, done, failed int) {
	var q string
	args := []interface{}{}
	if batchID != "" {
		q = `SELECT COUNT(*), COALESCE(SUM(status='queued'),0), COALESCE(SUM(status='running'),0), COALESCE(SUM(status='done'),0), COALESCE(SUM(status='error'),0) FROM downloads WHERE batch_id=?`
		args = append(args, batchID)
	} else {
		q = `SELECT COUNT(*), COALESCE(SUM(status='queued'),0), COALESCE(SUM(status='running'),0), COALESCE(SUM(status='done'),0), COALESCE(SUM(status='error'),0) FROM downloads`
	}
	db.QueryRow(q, args...).Scan(&total, &queued, &running, &done, &failed)
	return
}
