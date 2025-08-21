FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod .
RUN go mod download
COPY . .
RUN go build -o collector ./cmd/collector

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/collector /app/collector
ENV LOG_SERVER_URL=http://log-server:8080/ingest
ENV UDP_ADDR=:5140
EXPOSE 5140/udp
ENTRYPOINT ["/app/collector"]
