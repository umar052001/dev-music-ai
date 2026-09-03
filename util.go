package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// writeJSON encodes v to w as JSON, logging any encoding error.
func writeJSON(w http.ResponseWriter, v interface{}) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

// min returns the smaller of a and b.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// maxStr limits a string to n characters (useful for logs).
func maxStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// getString safely reads a string field from a decoded JSON map.
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getFloat safely reads a float64 (or numeric-string) field from a map.
func getFloat(m map[string]interface{}, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	return 0
}

// atoiDefault parses s as an int, returning def on failure or empty input.
func atoiDefault(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}

// corsMiddleware adds permissive CORS headers (fine for a local app) and
// short-circuits preflight OPTIONS requests.
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

// securityHeaders sets sensible response security headers on every response.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// spaHandler serves the built frontend, falling back to index.html for any
// path that isn't a real file (single-page app routing). The resolved path is
// validated to stay inside frontendDir so a crafted URL cannot read files
// outside the built assets.
func spaHandler(w http.ResponseWriter, r *http.Request) {
	clean := filepath.Clean(r.URL.Path)
	if strings.HasPrefix(clean, "/../") {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(frontendDir, filepath.FromSlash(clean))
	frontendRoot := filepath.Join(filepath.Clean(frontendDir))
	if path != frontendRoot && !strings.HasPrefix(path, frontendRoot+string(os.PathSeparator)) {
		http.NotFound(w, r)
		return
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		http.ServeFile(w, r, path)
		return
	}
	http.ServeFile(w, r, filepath.Join(frontendDir, "index.html"))
}
