FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod .
RUN go mod download
COPY . .
RUN go build -o sim ./cmd/sim

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/sim /app/sim
ENV COLLECTOR_ADDR=localhost:5140
ENTRYPOINT ["/app/sim"]
