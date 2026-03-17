package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

//go:embed web
var webFS embed.FS

var store *ScoreStore
var nickRe = regexp.MustCompile(`^[A-Za-z0-9]{3}$`)

const maxScoreLevel = 5

func main() {
	store = NewScoreStore(os.Getenv("DB_PATH"))

	// Ensure .wasm gets correct MIME type (some systems lack it)
	_ = mime.AddExtensionType(".wasm", "application/wasm")
	_ = mime.AddExtensionType(".ogg", "audio/ogg")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8070"
	}

	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/scores", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			handleGetScores(w, r)
		case http.MethodPost:
			handlePostScore(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})
	mux.Handle("/", http.FileServer(http.FS(sub)))

	addr := ":" + port
	log.Printf("CARGO SHIFT → http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func handleGetScores(w http.ResponseWriter, r *http.Request) {
	n := 10
	if nStr := r.URL.Query().Get("n"); nStr != "" {
		if v, err := strconv.Atoi(nStr); err == nil && v > 0 && v <= 50 {
			n = v
		}
	}
	entries := store.Top(n)
	if entries == nil {
		entries = []ScoreEntry{}
	}
	_ = json.NewEncoder(w).Encode(entries)
}

func handlePostScore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Nick  string `json:"nick"`
		Score int    `json:"score"`
		Level int    `json:"level"`
		Lines int    `json:"lines"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}

	nick := strings.ToUpper(strings.TrimSpace(req.Nick))
	if !nickRe.MatchString(nick) {
		http.Error(w, `{"error":"nick must be 3 letters or digits"}`, http.StatusBadRequest)
		return
	}
	if req.Score < 0 || req.Score > 999999 {
		http.Error(w, `{"error":"score out of range"}`, http.StatusBadRequest)
		return
	}
	if req.Level < 1 || req.Level > maxScoreLevel {
		http.Error(w, `{"error":"invalid level"}`, http.StatusBadRequest)
		return
	}
	if req.Lines < 0 || req.Lines > 9999 {
		http.Error(w, `{"error":"lines out of range"}`, http.StatusBadRequest)
		return
	}

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	entry := ScoreEntry{
		Nick:  nick,
		Score: req.Score,
		Level: req.Level,
		Lines: req.Lines,
	}
	msg, status := store.Add(entry, ip)
	if msg != "" {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
		return
	}
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(`{"ok":true}`))
}
