package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net"
	"os"
	"time"
)

type packet struct {
	Message string `json:"message"`
}

func main() {
	addr := os.Getenv("COLLECTOR_ADDR")
	if addr == "" {
		addr = "127.0.0.1:5140"
	}
	rand.Seed(time.Now().UnixNano())

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Fatal(err)
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	hosts := []string{"aiops9242", "web-01", "db-02"}
	users := []string{"root", "motadata", "alice", "baduser"}

	log.Printf("linux-sim sending to %s", addr)
	for {
		host := hosts[rand.Intn(len(hosts))]
		user := users[rand.Intn(len(users))]

		messages := []string{
			"<86> " + host + " sudo: pam_unix(sudo:session): session opened for user " + user + "(uid=0)",
			"<45> " + host + " CRON[123]: (root) CMD (run-parts /etc/cron.hourly)",
			"<12> " + host + " sshd[2222]: Failed password for " + user + " from 10.0.0.1 port 55321 ssh2",
			"<70> " + host + " systemd[1]: Started Session 123 of user " + user,
		}
		msg := messages[rand.Intn(len(messages))]
		payload, _ := json.Marshal(packet{Message: msg})
		_, _ = conn.Write(payload)

		time.Sleep(time.Duration(1000+rand.Intn(1000)) * time.Millisecond)
	}
}
