package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// cleanTitle asks the AI to turn a messy YouTube title into clean song
// metadata (title/artist/album). Falls back to a regex cleanup on error.
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
		return TitleCleanResponse{Title: regexCleanTitle(rawTitle)}
	}
	return parseTitleCleanResponse(resp, rawTitle)
}

// regexCleanTitle is a deterministic fallback used when the AI is unavailable.
func regexCleanTitle(raw string) string {
	cleaned := regexp.MustCompile(`\s*[\|–\-]\s*(Official|Music|Video|Audio|Lyrics|HD|4K|Visualizer).*`).ReplaceAllString(raw, "")
	cleaned = regexp.MustCompile(`\s*[\|–\-]\s*[A-Z][a-z]+ Music.*`).ReplaceAllString(cleaned, "")
	return strings.TrimSpace(cleaned)
}

// parseTitleCleanResponse extracts a TitleCleanResponse from an AI response,
// tolerating surrounding text.
func parseTitleCleanResponse(resp, fallbackTitle string) TitleCleanResponse {
	resp = strings.TrimSpace(resp)
	if idx := strings.Index(resp, "{"); idx >= 0 {
		if endIdx := strings.LastIndex(resp, "}"); endIdx > idx {
			resp = resp[idx : endIdx+1]
		}
	}
	var result TitleCleanResponse
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return TitleCleanResponse{Title: fallbackTitle}
	}
	if result.Title == "" {
		result.Title = fallbackTitle
	}
	return result
}

// getAISuggestions asks the AI for search suggestions based on the user's
// history. Falls back to deterministic suggestions on error.
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

// getFallbackSuggestions returns simple suggestions based on top artists.
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

// generatePlaylistSuggestion asks the AI for a playlist idea (name + search
// query) from a one-line description. Falls back to defaults on error.
func generatePlaylistSuggestion(description string) PlaylistSuggestion {
	prompt := fmt.Sprintf(`Generate a music playlist suggestion. Return ONLY valid JSON.

User request: "%s"

Return format: {"name": "playlist name", "query": "YouTube search query to find these songs", "track_count": 15, "mood": "mood description"}

Make the query specific enough to find good results. Include genre, era, or mood keywords.`, description)

	resp, err := AskLLM("", prompt, 0.6, false)
	if err != nil {
		return PlaylistSuggestion{Name: description, Query: description, Tracks: 10, Mood: "varied"}
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
