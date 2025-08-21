package main

import "testing"

func TestSeverityMap(t *testing.T) {
	if severityCodeToLevel(10) != "ERROR" {
		t.Fatalf("expected ERROR")
	}
	if severityCodeToLevel(40) != "WARN" {
		t.Fatalf("expected WARN")
	}
	if severityCodeToLevel(90) != "INFO" {
		t.Fatalf("expected INFO")
	}
}

func TestExtractBasic(t *testing.T) {
	msg := "<86> aiops9242 sudo: pam_unix(sudo:session): session opened for user root(uid=0)"
	rec := extract(msg)
	if rec.Hostname != "aiops9242" {
		t.Fatalf("hostname parse failed: %s", rec.Hostname)
	}
	if rec.EventSource != "linux" {
		t.Fatalf("source parse failed")
	}
	if !rec.IsBlacklisted {
		t.Fatalf("blacklist check failed")
	}
}
