# Build stage
FROM golang:1.26-alpine AS builder
# APK_MIRROR: 国内用 mirrors.aliyun.com 加速；海外留空用默认
ARG APK_MIRROR=mirrors.aliyun.com
RUN if [ -n "$APK_MIRROR" ]; then sed -i "s/dl-cdn.alpinelinux.org/$APK_MIRROR/g" /etc/apk/repositories; fi && \
    apk add --no-cache gcc musl-dev ca-certificates
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download 2>/dev/null || true

COPY . .

# Build a single Docker image that already contains LanceDB support.
RUN ECHO_FADE_MEMORY_HOME=/tmp/.echo-fade-memory go run ./cmd/setup-lancedb --static
RUN ARCH=$(go env GOARCH) && \
    CGO_ENABLED=1 \
    CGO_CFLAGS="-I/tmp/.echo-fade-memory/include" \
    CGO_LDFLAGS="/tmp/.echo-fade-memory/lib/linux_${ARCH}/liblancedb_go.a -ldl -lm -lpthread" \
    go build -tags lancedb -o echo-fade-memory ./cmd/echo-fade-memory

# Run stage
FROM alpine:3.19
ARG APK_MIRROR=mirrors.aliyun.com
RUN if [ -n "$APK_MIRROR" ]; then sed -i "s/dl-cdn.alpinelinux.org/$APK_MIRROR/g" /etc/apk/repositories; fi && \
    apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=builder /app/echo-fade-memory .

ENV HOME=/root
ENV ECHO_FADE_MEMORY_HOME=/root/.echo-fade-memory
RUN mkdir -p /root/.echo-fade-memory
VOLUME /root/.echo-fade-memory

EXPOSE 8080

ENTRYPOINT ["./echo-fade-memory"]
CMD ["serve"]
