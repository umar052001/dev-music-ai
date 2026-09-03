package main

import "time"

// SearchResult is a single song found by a YouTube search.
type SearchResult struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	CleanTitle string  `json:"clean_title,omitempty"`
	Uploader   string  `json:"uploader"`
	Duration   float64 `json:"duration"`
	URL        string  `json:"url"`
}

type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Count   int            `json:"count"`
}

// TrackFile is a local audio file belonging to the library.
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

type ActivityEntry struct {
	ID        int64  `json:"id"`
	Action    string `json:"action"`
	Track     string `json:"track"`
	Artist    string `json:"artist"`
	Query     string `json:"query"`
	CreatedAt string `json:"created_at"`
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
	Name   string `json:"name"`
	Query  string `json:"query"`
	Tracks int    `json:"track_count"`
	Mood   string `json:"mood"`
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

// PlaylistTrack is a single song inside an AI-generated playlist built from
// the user's own downloaded library.
type PlaylistTrack struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Path   string `json:"path"`
}

// LibraryPlaylist is an AI-suggested playlist made entirely from the user's
// downloaded songs.
type LibraryPlaylist struct {
	Name   string          `json:"name"`
	Mood   string          `json:"mood"`
	Tracks []PlaylistTrack `json:"tracks"`
}
