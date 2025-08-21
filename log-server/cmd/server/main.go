package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// LogRecord represents the normalized log structure.
type LogRecord struct {
	Timestamp     time.Time `json:"timestamp"`
	EventCategory string    `json:"event.category"`
	EventSource   string    `json:"event.source.type"`
	Username      string    `json:"username,omitempty"`
	Hostname      string    `json:"hostname,omitempty"`
	Severity      string    `json:"severity,omitempty"`
	Service       string    `json:"service,omitempty"`
	RawMessage    string    `json:"raw.message"`
	IsBlacklisted bool      `json:"is.blacklisted,omitempty"`
}

type ingestPayload LogRecord

// Storage interface (can be swapped with DB implementations).
type Store interface {
	Append(LogRecord) error
	All() ([]LogRecord, error)
	Count() int
	GroupBy(key string) map[string]int
}

type fileStore struct {
	path   string
	mu     sync.RWMutex
	cache  []LogRecord
	counts map[string]map[string]int // key -> value -> count
}

func newFileStore(path string) *fileStore {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	return &fileStore{
		path:   path,
		counts: map[string]map[string]int{"event.category": {}, "severity": {}},
	}
}

func (fs *fileStore) Append(rec LogRecord) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	f, err := os.OpenFile(fs.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}

	fs.cache = append(fs.cache, rec)
	fs.counts["event.category"][rec.EventCategory]++
	fs.counts["severity"][strings.ToUpper(rec.Severity)]++
	return nil
}

func (fs *fileStore) All() ([]LogRecord, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	out := make([]LogRecord, len(fs.cache))
	copy(out, fs.cache)
	return out, nil
}

func (fs *fileStore) Count() int {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return len(fs.cache)
}

func (fs *fileStore) GroupBy(key string) map[string]int {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	out := map[string]int{}
	if m, ok := fs.counts[key]; ok {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

// API server
type api struct {
	store *fileStore
}

func (a *api) ingest(w http.ResponseWriter, r *http.Request) {
	var p ingestPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	rec := LogRecord(p)
	if rec.Timestamp.IsZero() {
		rec.Timestamp = time.Now().UTC()
	}
	if rec.RawMessage == "" {
		http.Error(w, "raw.message required", http.StatusBadRequest)
		return
	}
	if err := a.store.Append(rec); err != nil {
		http.Error(w, "failed to store", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func parseBoolQuery(r *http.Request, key string) (*bool, error) {
	if v := r.URL.Query().Get(key); v != "" {
		b, err := strconv.ParseBool(v)
		return &b, err
	}
	return nil, nil
}

func (a *api) logs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	level := strings.TrimSpace(strings.ToUpper(q.Get("level")))
	service := strings.TrimSpace(q.Get("service"))
	username := strings.TrimSpace(q.Get("username"))
	isBlkPtr, err := parseBoolQuery(r, "is.blacklisted")
	if err != nil {
		http.Error(w, "is.blacklisted must be true/false", http.StatusBadRequest)
		return
	}

	limit := 0
	if s := q.Get("limit"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			limit = v
		}
	}
	sortKey := q.Get("sort")

	recs, _ := a.store.All()
	filtered := make([]LogRecord, 0, len(recs))
	for _, r := range recs {
		if level != "" && strings.ToUpper(r.Severity) != level {
			continue
		}
		if service != "" && r.Service != service {
			continue
		}
		if username != "" && r.Username != username {
			continue
		}
		if isBlkPtr != nil && r.IsBlacklisted != *isBlkPtr {
			continue
		}
		filtered = append(filtered, r)
	}

	switch sortKey {
	case "timestamp":
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].Timestamp.Before(filtered[j].Timestamp) })
	}

	if limit > 0 && limit < len(filtered) {
		filtered = filtered[:limit]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filtered)
}

func (a *api) metrics(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"total":           a.store.Count(),
		"groupByCategory": a.store.GroupBy("event.category"),
		"groupBySeverity": a.store.GroupBy("severity"),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func loadExisting(path string, fs *fileStore) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	for dec.More() {
		var rec LogRecord
		if err := dec.Decode(&rec); err != nil {
			return err
		}
		_ = fs.Append(rec)
	}
	return nil
}

func main() {
	store := newFileStore("data/logs.jsonl")
	_ = loadExisting("data/logs.jsonl", store)

	a := &api{store: store}
	r := mux.NewRouter()
	r.HandleFunc("/ingest", a.ingest).Methods("POST")
	r.HandleFunc("/logs", a.logs).Methods("GET")
	r.HandleFunc("/metrics", a.metrics).Methods("GET")

	addr := ":8080"
	log.Printf("log-server listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}
