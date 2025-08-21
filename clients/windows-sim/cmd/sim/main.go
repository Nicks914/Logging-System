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

	hosts := []string{"WIN-EQ5V3RA5F7H", "WIN-LAB-01", "WIN-APP-02"}
	users := []string{"Administrator", "Motadata", "Bob"}

	log.Printf("windows-sim sending to %s", addr)
	for {
		host := hosts[rand.Intn(len(hosts))]
		user := users[rand.Intn(len(users))]

		messages := []string{
			"<134> " + host + " Microsoft-Windows-Security-Auditing: A user account was successfully logged on. Account Name: " + user,
			"<22> " + host + " Microsoft-Windows-EventLog: Application Error occurred in Service Control Manager",
			"<41> " + host + " Microsoft-Windows-Security-Auditing: An account failed to log on. Account Name: " + user,
		}
		msg := messages[rand.Intn(len(messages))]
		payload, _ := json.Marshal(packet{Message: msg})
		_, _ = conn.Write(payload)

		time.Sleep(time.Duration(1000+rand.Intn(1000)) * time.Millisecond)
	}
}
