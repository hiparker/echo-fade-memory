# Build stage — pure Go, no CGO overhead
FROM golang:1.26-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app

COPY go.mod go.sum ./
COPY . .

RUN CGO_ENABLED=1 go build -o echo-fade-memory ./cmd/echo-fade-memory

# Run stage
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=builder /app/echo-fade-memory .

ENV HOME=/root
ENV ECHO_FADE_MEMORY_HOME=/root/.echo-fade-memory
RUN mkdir -p /root/.echo-fade-memory
VOLUME /root/.echo-fade-memory

EXPOSE 8080

ENTRYPOINT ["./echo-fade-memory"]
CMD ["serve"]
