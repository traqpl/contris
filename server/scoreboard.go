package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type ScoreEntry struct {
	Nick      string `json:"nick"`
	Score     int    `json:"score"`
	Level     int    `json:"level"`
	Lines     int    `json:"lines"`
	Timestamp string `json:"timestamp"`
}

type ScoreStore struct {
	db     *sql.DB
	mu     sync.Mutex
	lastIP map[string]time.Time
}

func NewScoreStore(dbPath string) *ScoreStore {
	if dbPath == "" {
		dbPath = "scores.db"
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatalf("scores db dir: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open scores db %s: %v", dbPath, err)
	}
	db.SetMaxOpenConns(1)

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS scores (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		nick      TEXT    NOT NULL,
		score     INTEGER NOT NULL,
		level     INTEGER NOT NULL,
		lines     INTEGER NOT NULL,
		timestamp TEXT    NOT NULL
	)`)
	if err != nil {
		log.Fatalf("create scores table: %v", err)
	}

	log.Printf("scores db: %s", dbPath)
	return &ScoreStore{db: db, lastIP: make(map[string]time.Time)}
}

func (s *ScoreStore) Top(n int) []ScoreEntry {
	rows, err := s.db.Query(
		`SELECT nick, score, level, lines, timestamp
		 FROM scores
		 ORDER BY score DESC, level DESC, lines DESC, timestamp ASC
		 LIMIT ?`, n,
	)
	if err != nil {
		log.Printf("scores query: %v", err)
		return nil
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("scores close rows: %v", err)
		}
	}()

	var entries []ScoreEntry
	for rows.Next() {
		var e ScoreEntry
		if err := rows.Scan(&e.Nick, &e.Score, &e.Level, &e.Lines, &e.Timestamp); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		log.Printf("scores rows iteration: %v", err)
		return nil
	}
	return entries
}

func (s *ScoreStore) Add(entry ScoreEntry, ip string) (string, int) {
	s.mu.Lock()
	last, ok := s.lastIP[ip]
	if ok && time.Since(last) < 5*time.Second {
		s.mu.Unlock()
		return "too many requests", http.StatusTooManyRequests
	}
	s.lastIP[ip] = time.Now()
	s.mu.Unlock()

	entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO scores (nick, score, level, lines, timestamp) VALUES (?, ?, ?, ?, ?)`,
		entry.Nick, entry.Score, entry.Level, entry.Lines, entry.Timestamp,
	)
	if err != nil {
		log.Printf("scores insert: %v", err)
		return "db error", http.StatusInternalServerError
	}
	return "", 0
}
