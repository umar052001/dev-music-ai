package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	ytdlp        = "yt-dlp"
	audioQuality = "320K"
)

// musicExtSet lists the audio extensions considered part of the library.
var musicExtSet = map[string]bool{
	".mp3": true, ".m4a": true, ".flac": true,
	".wav": true, ".ogg": true, ".opus": true, ".aac": true,
}

// dlRoot is the absolute path where downloaded music is stored.
var dlRoot string

// safeName converts an arbitrary string into a filesystem-friendly folder/file
// name, replacing illegal characters and trimming stray dots/spaces.
func safeName(s string) string {
	re := regexp.MustCompile(`[\\/:*?"<>|\n\r\t]`)
	s = re.ReplaceAllString(s, "_")
	s = strings.Trim(s, ". ")
	if s == "" {
		return "Unknown"
	}
	return s
}

// outputTemplate builds the yt-dlp output pattern for the requested layout.
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

// collectTracks reads the audio files in a directory into TrackFile entries.
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

// syncDBWithDisk scans the download folder and records any new tracks in the
// database so they appear in the library and AI playlists.
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

// searchYouTube runs a yt-dlp search and returns up to limit results.
func searchYouTube(query string, limit int) ([]SearchResult, error) {
	cmd := exec.Command(ytdlp,
		"--flat-playlist", "--dump-json", "--no-warnings",
		fmt.Sprintf("ytsearch%d:%s", limit, query),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp search: %w: %s", err, string(out))
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
		url := getString(obj, "webpage_url")
		if url == "" && id != "" {
			url = "https://www.youtube.com/watch?v=" + id
		}
		results = append(results, SearchResult{
			ID:       id,
			Title:    getString(obj, "title"),
			Uploader: getString(obj, "uploader"),
			Duration: getFloat(obj, "duration"),
			URL:      url,
		})
	}
	return results, nil
}

// resolveYouTubeURL searches for a single song and returns its watch URL.
func resolveYouTubeURL(title, artist string) string {
	q := title
	if artist != "" {
		q = title + " " + artist
	}
	cmd := exec.Command(ytdlp, "--flat-playlist", "--dump-json", "--no-warnings",
		"--extractor-args", "youtube:player_client=web_embedded",
		fmt.Sprintf("ytsearch1:%s", q))
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	var obj map[string]interface{}
	if json.Unmarshal(out, &obj) != nil {
		return ""
	}
	if id := getString(obj, "id"); id != "" {
		return "https://www.youtube.com/watch?v=" + id
	}
	return ""
}

// audioMime returns the MIME type for a container extension.
func audioMime(ext string) string {
	switch ext {
	case "m4a", "mp4":
		return "audio/mp4"
	case "mp3":
		return "audio/mpeg"
	case "ogg", "opus":
		return "audio/ogg"
	default:
		return "audio/webm"
	}
}

// streamAudio writes the selected video's audio to the response, optionally
// re-encoding to MP3. It uses progressive flushing for instant playback.
func streamAudio(w http.ResponseWriter, r *http.Request) {
	videoURL := r.URL.Query().Get("url")
	if videoURL == "" {
		http.Error(w, "url param required", http.StatusBadRequest)
		return
	}

	// Determine the actual container/extension so the browser gets a correct
	// Content-Type. Default to webm (opus).
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

	mime := audioMime(ext)
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Accept-Ranges", "none")

	// Force re-encode to MP3 when requested (universally playable), otherwise
	// stream the native audio for an instant start.
	var args []string
	if r.URL.Query().Get("format") == "mp3" {
		mime = "audio/mpeg"
		w.Header().Set("Content-Type", mime)
		args = []string{"-x", "--audio-format", "mp3", "--audio-quality", audioQuality}
	}
	args = append(args,
		"--no-playlist", "--no-warnings", "-o", "-",
		"--extractor-args", "youtube:player_client=web_embedded",
		"-f", "bestaudio/best", videoURL,
	)
	cmd := exec.Command(ytdlp, args...)

	cmd.Stdout = &flushWriter{w: w, f: flusher}
	cmd.Stderr = os.Stderr
	log.Printf("streaming (%s): %s", mime, videoURL)
	if err := cmd.Run(); err != nil {
		log.Printf("stream error: %v", err)
	}
}

// flushWriter flushes after each write for progressive streaming.
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

// serveAudioFile serves a local audio file from downloadRoot, setting the
// correct MIME type and enabling range requests. The requested path is
// validated against dlRoot with os.Root so a crafted path cannot escape the
// download directory (path-traversal safe).
func serveAudioFile(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/api/file/")
	if rel == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}
	// os.Root resolves relative paths inside dlRoot and rejects attempts to
	// escape it (e.g. ".." segments), preventing arbitrary file reads.
	root, err := os.OpenRoot(dlRoot)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer root.Close()

	f, err := root.Open(rel)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	contentTypes := map[string]string{
		".mp3": "audio/mpeg", ".m4a": "audio/mp4", ".flac": "audio/flac",
		".wav": "audio/wav", ".ogg": "audio/ogg", ".opus": "audio/opus", ".aac": "audio/aac",
	}
	if ct, ok := contentTypes[strings.ToLower(filepath.Ext(info.Name()))]; ok {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Accept-Ranges", "bytes")
	// ServeContent handles Range requests and sets Last-Modified/Content-Length.
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// buildLibrary returns the download folder grouped by artist and album.
func buildLibrary() LibraryResponse {
	lib := LibraryResponse{Root: dlRoot, Artists: []ArtistNode{}}
	entries, err := os.ReadDir(dlRoot)
	if err != nil {
		return lib
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
	return lib
}

// allSongs returns every track in the library, sorted by name.
func allSongs() AllSongsResponse {
	resp := AllSongsResponse{Songs: []TrackFile{}}
	entries, err := os.ReadDir(dlRoot)
	if err != nil {
		return resp
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		artistName := entry.Name()
		resp.Songs = append(resp.Songs, collectTracks(filepath.Join(dlRoot, artistName), artistName, "Singles")...)
		subEntries, _ := os.ReadDir(filepath.Join(dlRoot, artistName))
		for _, sub := range subEntries {
			if !sub.IsDir() {
				continue
			}
			resp.Songs = append(resp.Songs, collectTracks(filepath.Join(dlRoot, artistName, sub.Name()), artistName, sub.Name())...)
		}
	}
	sort.Slice(resp.Songs, func(i, j int) bool { return resp.Songs[i].Name < resp.Songs[j].Name })
	for _, s := range resp.Songs {
		resp.Total += s.Size
	}
	resp.Count = len(resp.Songs)
	return resp
}

// paginateSongs applies page/limit pagination to a full song list.
func paginateSongs(songs []TrackFile, page, limit int) []TrackFile {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	start := (page - 1) * limit
	if start >= len(songs) {
		return []TrackFile{}
	}
	end := start + limit
	if end > len(songs) {
		end = len(songs)
	}
	return songs[start:end]
}


