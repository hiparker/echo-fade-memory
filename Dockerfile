# Build stage
FROM golang:1.26-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download 2>/dev/null || true

COPY . .
RUN CGO_ENABLED=0 go build -o echo-fade-memory ./cmd/echo-fade-memory

# Run stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=builder /app/echo-fade-memory .

ENV DATA_PATH=/data
VOLUME /data

EXPOSE 8080

ENTRYPOINT ["./echo-fade-memory"]
