package main

import (
	"testing"
	"time"
)

func TestFileStoreAppendAndAll(t *testing.T) {
	fs := newFileStore(t.TempDir() + "/logs.jsonl")
	rec := LogRecord{Timestamp: time.Now(), EventCategory: "login.audit", Severity: "INFO", RawMessage: "x"}
	if err := fs.Append(rec); err != nil {
		t.Fatalf("append: %v", err)
	}
	all, err := fs.All()
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1, got %d", len(all))
	}
	if fs.Count() != 1 {
		t.Fatalf("count mismatch")
	}
	gb := fs.GroupBy("event.category")
	if gb["login.audit"] != 1 {
		t.Fatalf("group by category failed")
	}
}
