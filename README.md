# Centralized Logging System — Golang Microservices + Docker

This repo implements a simple centralized logging system composed of:
- **Client microservices**: simulate Linux & Windows logs and transmit to a **log-collector** over UDP.
- **log-collector**: receives logs, parses/enriches, and forwards them to the **log-server** over HTTP.
- **log-server**: stores logs (JSONL file-based) and exposes query + metrics APIs.

## Architecture

```

                                  +-------------------+       HTTP POST /ingest
+------------------+              |   log-collector   |  ----------------------->  +------------------+
| Windows Client   |  ----> UDP   |  (UDP listener)   |                            |    log-server    |
+------------------+              +-------------------+                            | (file storage)   |
                                                                                   +------------------+
                                                                                   | GET /logs        |
                                                                                   | GET /metrics     |
                                                                                   +------------------+
```

- Transport: **UDP** from clients to collector for simplicity & speed.
- Forwarding: **HTTP** from collector to server.
- Storage: **JSONL** file (`data/logs.jsonl`), with in-memory indices built per process lifecycle.

## Quick Start

From within a service directory (log-server)
```bash
go mod tidy       # cleans and adds missing modules
```
Build Docker
```bash
docker compose up --build
```

Once Docker up:
- `log-server` APIs:
  - `GET http://localhost:8080/logs?limit=10&sort=timestamp`
  - `GET http://localhost:8080/logs?level=error`
  - `GET http://localhost:8080/logs?service=linux_login&level=warn`
  - `GET http://localhost:8080/logs?username=root&is.blacklisted=true`
  - `GET http://localhost:8080/metrics`

You should see the clients generating logs every ~1–2 seconds, the collector forwarding, and the server storing.

## Services

- `clients/linux-sim`: Simulates **Syslog** + **Login Audit** logs.
- `clients/windows-sim`: Simulates **Windows Login Audit** + **Event Logs**.
- `log-collector`: UDP listener, parser, enricher (blacklist), HTTP forwarder.
- `log-server`: Ingest & Query API + Metrics.

## Testing

From within each service directory:
```bash
go test ./...
```

## Notes
- The parser is intentionally simple (regex/string parsing) to satisfy the assignment without heavy dependencies.
- Extendable storage via the `storage.Store` interface in `log-server`.
