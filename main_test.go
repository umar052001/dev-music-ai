package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// init sets package globals that some pure functions (e.g. outputTemplate)
// depend on so tests don't touch the real download folder.
func TestMain(m *testing.M) {
	dir, _ := os.MkdirTemp("", "devmusic-test")
	dlRoot = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestAudioMime(t *testing.T) {
	cases := map[string]string{
		"m4a":  "audio/mp4",
		"mp4":  "audio/mp4",
		"mp3":  "audio/mpeg",
		"ogg":  "audio/ogg",
		"opus": "audio/ogg",
		"webm": "audio/webm",
		"flac": "audio/webm", // unknown containers fall back to webm
	}
	for in, want := range cases {
		if got := audioMime(in); got != want {
			t.Errorf("audioMime(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSafeNameRemovesIllegalChars(t *testing.T) {
	got := safeName(`Song/Name:*?"<>|`)
	if got != "Song_Name_______" {
		t.Errorf("safeName = %q", got)
	}
}

func TestSafeNameTrimsDots(t *testing.T) {
	if got := safeName("  .Title.  "); got != "Title" {
		t.Errorf("safeName = %q, want Title", got)
	}
}

func TestSafeNameEmptyFallsBack(t *testing.T) {
	if got := safeName("..."); got != "Unknown" {
		t.Errorf("safeName = %q, want Unknown", got)
	}
}

func TestOutputTemplateArtistAlbum(t *testing.T) {
	got := outputTemplate("artist_album", "Artist", "Album")
	want := filepath.Join(dlRoot, "Artist", "Album", "%(track_number|0)02d - %(title)s.%(ext)s")
	if got != want {
		t.Errorf("outputTemplate = %q, want %q", got, want)
	}
}

func TestOutputTemplateArtistOnly(t *testing.T) {
	got := outputTemplate("artist_only", "Artist", "")
	want := filepath.Join(dlRoot, "Artist", "%(title)s.%(ext)s")
	if got != want {
		t.Errorf("outputTemplate = %q, want %q", got, want)
	}
}

func TestStripFileName(t *testing.T) {
	cases := map[string]string{
		"Song Name.mp3":                     "Song Name",
		"Song - Official Video.mp3":         "Song",
		"Song (Official Audio).mp4":         "Song",
		"Song Name - 1080p.mp3":             "Song Name - 1080p", // -1080 isn't stripped
		"Already Clean.m4a":                 "Already Clean",
	}
	for in, want := range cases {
		if got := stripFileName(in); got != want {
			t.Errorf("stripFileName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPaginateSongs(t *testing.T) {
	songs := make([]TrackFile, 25)
	for i := range songs {
		songs[i] = TrackFile{Name: string(rune('a' + i))}
	}
	if got := paginateSongs(songs, 1, 10); len(got) != 10 {
		t.Errorf("page 1 len = %d, want 10", len(got))
	}
	if got := paginateSongs(songs, 3, 10); len(got) != 5 {
		t.Errorf("page 3 len = %d, want 5", len(got))
	}
	if got := paginateSongs(songs, 99, 10); len(got) != 0 {
		t.Errorf("out-of-range page len = %d, want 0", len(got))
	}
}

func TestMin(t *testing.T) {
	if got := min(3, 9); got != 3 {
		t.Errorf("min(3,9) = %d, want 3", got)
	}
	if got := min(9, 3); got != 3 {
		t.Errorf("min(9,3) = %d, want 3", got)
	}
}

func TestGetStringAndGetFloat(t *testing.T) {
	m := map[string]interface{}{"a": "x", "n": 42.0, "s": "7.5", "nil": nil}
	if got := getString(m, "a"); got != "x" {
		t.Errorf("getString = %q", got)
	}
	if got := getString(m, "missing"); got != "" {
		t.Errorf("getString missing = %q", got)
	}
	if got := getString(m, "nil"); got != "" {
		t.Errorf("getString nil = %q", got)
	}
	if got := getFloat(m, "n"); got != 42.0 {
		t.Errorf("getFloat numeric = %v", got)
	}
	if got := getFloat(m, "s"); got != 7.5 {
		t.Errorf("getFloat string = %v", got)
	}
	if got := getFloat(m, "missing"); got != 0 {
		t.Errorf("getFloat missing = %v", got)
	}
}

func TestRegexCleanTitle(t *testing.T) {
	got := regexCleanTitle(`Song Name - Official Video HD`)
	if got != "Song Name" {
		t.Errorf("regexCleanTitle = %q, want %q", got, "Song Name")
	}
}

func TestParseTitleCleanResponse(t *testing.T) {
	resp := `Here you go: {"title": "Clean Song", "artist": "Artist", "album": "Album"}`
	got := parseTitleCleanResponse(resp, "fallback")
	if got.Title != "Clean Song" || got.Artist != "Artist" || got.Album != "Album" {
		t.Errorf("parseTitleCleanResponse = %+v", got)
	}
}

func TestParseTitleCleanResponseInvalid(t *testing.T) {
	// Invalid JSON should fall back to the given title.
	if got := parseTitleCleanResponse("not json", "Fallback Title"); got.Title != "Fallback Title" {
		t.Errorf("parseTitleCleanResponse invalid = %+v", got)
	}
}

func TestParseBatchFallback(t *testing.T) {
	raw := "[Title One](https://youtu.be/abc) — Artist A\n" +
		"Title Two - Artist B\n" +
		"https://youtu.be/xyz Title Three by Artist C\n" +
		"Only a title\n" +
		"\n"
	got := parseBatchFallback(raw)
	if len(got) != 4 {
		t.Fatalf("parseBatchFallback len = %d, want 4: %+v", len(got), got)
	}
	if got[0].Title != "Title One" || got[0].Artist != "Artist A" || got[0].URL != "https://youtu.be/abc" {
		t.Errorf("entry 0 = %+v", got[0])
	}
	if got[1].Title != "Title Two" || got[1].Artist != "Artist B" {
		t.Errorf("entry 1 = %+v", got[1])
	}
	if got[2].Title != "Title Three" || got[2].Artist != "Artist C" {
		t.Errorf("entry 2 = %+v", got[2])
	}
	if got[3].Artist != "" {
		t.Errorf("entry 3 should have no artist: %+v", got[3])
	}
}

func TestParseBatchFallbackDedupes(t *testing.T) {
	got := parseBatchFallback("Song - Artist\nSong - Artist")
	if len(got) != 1 {
		t.Errorf("expected dedupe, got %d", len(got))
	}
}

func TestParseBatchJSON(t *testing.T) {
	resp := "```json\n[{\"title\":\"A\",\"artist\":\"X\",\"url\":\"1\"},{\"title\":\"B\",\"artist\":\"Y\",\"url\":\"2\"}]\n```"
	got := parseBatchJSON(resp)
	if len(got) != 2 {
		t.Fatalf("parseBatchJSON len = %d, want 2", len(got))
	}
	if got[0].Title != "A" || got[1].Title != "B" {
		t.Errorf("parseBatchJSON = %+v", got)
	}
}

func TestParseBatchJSONTruncated(t *testing.T) {
	// Truncated/malformed JSON should return an empty slice, not panic.
	resp := `[{"title":"A","artist":"X","url":"1"},{"title":"B"`
	if got := parseBatchJSON(resp); len(got) != 0 {
		t.Errorf("truncated should produce empty, got %+v", got)
	}
}

func TestMapPlaylistResponse(t *testing.T) {
	songs := []librarySongSummary{
		{name: "Artist A - Song One", track: TrackFile{Path: "a/song1.mp3", Name: "Song One", Artist: "Artist A"}},
		{name: "Artist B - Song Two", track: TrackFile{Path: "b/song2.mp3", Name: "Song Two", Artist: "Artist B"}},
	}
	resp := `{"name":"My Mix","mood":"chill","tracks":["Artist B - Song Two","Artist A - Song One","Unknown Song"]}`
	pl, err := mapPlaylistResponse(resp, "desc", songs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pl.Name != "My Mix" {
		t.Errorf("name = %q", pl.Name)
	}
	if len(pl.Tracks) != 2 {
		t.Fatalf("tracks len = %d, want 2: %+v", len(pl.Tracks), pl.Tracks)
	}
	// Order follows the AI's chosen order.
	if pl.Tracks[0].Title != "Song Two" || pl.Tracks[1].Title != "Song One" {
		t.Errorf("order wrong: %+v", pl.Tracks)
	}
	if pl.Tracks[0].Path != "b/song2.mp3" {
		t.Errorf("path wrong: %+v", pl.Tracks[0])
	}
}

func TestMapPlaylistResponseUnknownSongs(t *testing.T) {
	songs := []librarySongSummary{{name: "A - B", track: TrackFile{Path: "x"}}}
	_, err := mapPlaylistResponse(`{"name":"x","tracks":["Does Not Exist"]}`, "desc", songs)
	if err == nil {
		t.Error("expected error when no tracks match")
	}
}

func TestCollectLibrarySongsEmpty(t *testing.T) {
	// In the temp dir there are no songs, so the collection should be empty.
	if got := collectLibrarySongs(); len(got) != 0 {
		t.Errorf("collectLibrarySongs should be empty, got %d", len(got))
	}
}

func TestPlaylistTrackTypeMatching(t *testing.T) {
	// Sanity check the fields used by the frontend JSON serialization.
	pt := PlaylistTrack{Title: "T", Artist: "A", Path: "/x.mp3"}
	got := reflect.TypeOf(pt).NumField()
	if got != 3 {
		t.Errorf("PlaylistTrack should have 3 JSON fields, has %d", got)
	}
}
