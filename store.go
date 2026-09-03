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
