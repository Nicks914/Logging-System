package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type incoming struct {
	Message string `json:"message"`
}

type outgoing struct {
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

var (
	// simplistic regexes adequate for our simulated messages
	reSeverity = regexp.MustCompile(`^<(\d+)>`)
	reHost     = regexp.MustCompile(`>\s*([A-Za-z0-9._-]+)\s+`)
	reUser     = regexp.MustCompile(`user\s+([A-Za-z0-9._-]+)`)
	reWinUser  = regexp.MustCompile(`Account Name:\s*([A-Za-z0-9._-]+)`)
)

var blacklistedUsers = map[string]bool{
	"root":          true,
	"Administrator": true,
	"baduser":       true,
}

func severityCodeToLevel(code int) string {
	// naive mapping
	switch {
	case code >= 0 && code < 30:
		return "ERROR"
	case code >= 30 && code < 60:
		return "WARN"
	default:
		return "INFO"
	}
}

func parseCategory(msg string) (string, string) {
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "login") && strings.Contains(lower, "success") {
		return "login.audit", "linux"
	}
	if strings.Contains(lower, "login") {
		return "login.audit", "linux"
	}
	if strings.Contains(lower, "logout") || strings.Contains(lower, "session terminated") {
		return "logout.audit", "linux"
	}
	if strings.Contains(lower, "security-auditing") {
		return "windows.login", "windows"
	}
	if strings.Contains(lower, "event log") || strings.Contains(lower, "application") {
		return "windows.event", "windows"
	}
	if strings.Contains(lower, "sudo") || strings.Contains(lower, "pam_unix") {
		return "syslog", "linux"
	}
	return "unknown", "unknown"
}

func extract(msg string) outgoing {
	out := outgoing{
		Timestamp:  time.Now().UTC(),
		RawMessage: msg,
	}

	// severity
	if m := reSeverity.FindStringSubmatch(msg); len(m) == 2 {
		// convert to int, rough mapping
		out.Severity = severityCodeToLevel(atoiSafe(m[1]))
	}

	// hostname
	if m := reHost.FindStringSubmatch(msg); len(m) == 2 {
		out.Hostname = m[1]
	}

	// username (linux/win forms)
	if m := reUser.FindStringSubmatch(msg); len(m) == 2 {
		out.Username = m[1]
	} else if m := reWinUser.FindStringSubmatch(msg); len(m) == 2 {
		out.Username = m[1]
	}

	// category & source
	cat, src := parseCategory(msg)
	out.EventCategory, out.EventSource = cat, src

	// service hint
	switch src {
	case "linux":
		if strings.Contains(strings.ToLower(cat), "login") {
			out.Service = "linux_login"
		} else if strings.Contains(strings.ToLower(cat), "logout") {
			out.Service = "linux_logout"
		} else {
			out.Service = "linux_syslog"
		}
	case "windows":
		if strings.Contains(strings.ToLower(cat), "login") {
			out.Service = "windows_login"
		} else {
			out.Service = "windows_event"
		}
	}

	// enrich
	if blacklistedUsers[out.Username] {
		out.IsBlacklisted = true
	}

	return out
}

func atoiSafe(s string) int {
	n := 0
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int(ch-'0')
		}
	}
	return n
}

func forward(serverURL string, rec outgoing) error {
	b, _ := json.Marshal(rec)
	req, err := http.NewRequest("POST", serverURL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return err
	}
	return nil
}

func main() {
	serverURL := getenv("LOG_SERVER_URL", "http://localhost:8080/ingest")
	udpAddr := getenv("UDP_ADDR", ":5140")

	addr, err := net.ResolveUDPAddr("udp", udpAddr)
	if err != nil {
		log.Fatalf("resolve UDP: %v", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("listen UDP: %v", err)
	}
	defer conn.Close()
	log.Printf("log-collector listening UDP on %s, forwarding to %s", udpAddr, serverURL)

	buf := make([]byte, 4096)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("read UDP: %v", err)
			continue
		}
		data := buf[:n]

		// handle each datagram concurrently
		go func(b []byte) {
			var in incoming
			if err := json.Unmarshal(b, &in); err != nil {
				log.Printf("bad json: %v", err)
				return
			}
			rec := extract(in.Message)
			if err := forward(serverURL, rec); err != nil {
				log.Printf("forward failed: %v", err)
			}
		}(append([]byte{}, data...))
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
