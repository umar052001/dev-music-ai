package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// libraryPlaylistError is used when the AI cannot build a playlist from the
// user's library (e.g. not enough songs, or AI unavailable).
const libraryPlaylistError = "Could not build a playlist from your library. Make sure you have downloaded some songs and that the AI provider is reachable."

// stripFileName removes the file extension and common yt-dlp artifacts so
// names are friendly for the AI and for matching.
func stripFileName(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	base = regexp.MustCompile(`(?i)\s*[-–—(]\s*(official\s*(video|audio)?|lyrics?|hd|4k|visualizer|audio)\s*\)?\s*$`).ReplaceAllString(base, "")
	return strings.TrimSpace(base)
}

// librarySongSummary is the working set given to the AI: our own local index
// plus a clean human-readable name for matching.
type librarySongSummary struct {
	index int
	name  string // clean display name: "Artist - Title"
	track TrackFile
}

// collectLibrarySongs returns a deterministic list of the user's downloaded
// songs with clean display names.
func collectLibrarySongs() []librarySongSummary {
	full := allSongs()
	summaries := make([]librarySongSummary, 0, len(full.Songs))
	for i := range full.Songs {
		t := full.Songs[i]
		display := stripFileName(t.Name)
		if t.Artist != "" {
			display = t.Artist + " - " + display
		}
		summaries = append(summaries, librarySongSummary{index: i, name: display, track: t})
	}
	return summaries
}

// buildLibraryPlaylist asks the AI to pick a themed playlist from the user's
// own downloaded songs and maps the result back to the real audio files.
func buildLibraryPlaylist(description string, trackCount int) (LibraryPlaylist, error) {
	songs := collectLibrarySongs()
	if len(songs) == 0 {
		return LibraryPlaylist{}, fmt.Errorf("no downloaded songs")
	}
	if trackCount < 1 {
		trackCount = 10
	}
	if trackCount > len(songs) {
		trackCount = len(songs)
	}

	var b strings.Builder
	for _, s := range songs {
		b.WriteString("- " + s.name + "\n")
	}

	prompt := fmt.Sprintf(`Here are the user's downloaded songs (the format is "Artist - Title"):

%s
The user wants a playlist for: "%s"

Pick %d songs from the list above that best fit this request. Build them into ONE cohesive playlist (good flow, order matters).

Return ONLY valid JSON: {"name": "playlist name", "mood": "short mood", "tracks": ["exact display name 1", "exact display name 2", ...]}

The "tracks" values MUST be copied EXACTLY from the list above (including "Artist - Title"). Do not invent songs. Return ONLY the JSON, no explanation.`, b.String(), description, trackCount)

	// Use the fast model (usually a small, reliable cloud/local model) for this
	// structured task; it keeps playlist generation quick and less likely to be
	// blocked by a paid-only smart model.
	resp, err := AskLLM("", prompt, 0.7, true, 4096)
	if err != nil {
		return LibraryPlaylist{}, err
	}
	return mapPlaylistResponse(resp, description, songs)
}

// mapPlaylistResponse parses the AI's JSON and resolves the chosen songs back
// to the user's actual library tracks, preserving the AI's chosen order.
func mapPlaylistResponse(resp, description string, songs []librarySongSummary) (LibraryPlaylist, error) {
	resp = strings.TrimSpace(resp)
	if idx := strings.Index(resp, "{"); idx >= 0 {
		if endIdx := strings.LastIndex(resp, "}"); endIdx > idx {
			resp = resp[idx : endIdx+1]
		}
	}

	var parsed struct {
		Name   string   `json:"name"`
		Mood   string   `json:"mood"`
		Tracks []string `json:"tracks"`
	}
	if err := json.Unmarshal([]byte(resp), &parsed); err != nil {
		return LibraryPlaylist{}, fmt.Errorf("ai returned invalid playlist json: %w", err)
	}

	byName := make(map[string]librarySongSummary, len(songs))
	for _, s := range songs {
		byName[s.name] = s
	}

	var picks []PlaylistTrack
	seen := map[string]bool{}
	for _, raw := range parsed.Tracks {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		s, ok := byName[name]
		if !ok {
			// Fall back to a case-insensitive substring match.
			for _, cand := range songs {
				if strings.EqualFold(strings.TrimSpace(cand.name), strings.TrimSpace(name)) {
					s, ok = cand, true
					break
				}
			}
		}
		if !ok || seen[s.track.Path] {
			continue
		}
		seen[s.track.Path] = true
		picks = append(picks, PlaylistTrack{
			Title:  stripFileName(s.track.Name),
			Artist: s.track.Artist,
			Path:   s.track.Path,
		})
	}
	if len(picks) == 0 {
		return LibraryPlaylist{}, fmt.Errorf("ai returned no usable tracks")
	}

	name := parsed.Name
	if name == "" {
		name = description
	}
	return LibraryPlaylist{Name: name, Mood: parsed.Mood, Tracks: picks}, nil
}
