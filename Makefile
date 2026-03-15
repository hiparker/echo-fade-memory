.PHONY: build setup-lancedb setup-lancedb-static build-lancedb run remember recall decay test test-lancedb print-runtime-paths clean docker-build

BINARY := echo-fade-memory
RUNTIME_HOME := $(if $(ECHO_FADE_MEMORY_HOME),$(ECHO_FADE_MEMORY_HOME),$(HOME)/.echo-fade-memory)
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_M),x86_64)
ARCH := amd64
else ifeq ($(UNAME_M),amd64)
ARCH := amd64
else ifeq ($(UNAME_M),arm64)
ARCH := arm64
else ifeq ($(UNAME_M),aarch64)
ARCH := arm64
else
ARCH := unsupported
endif

ifeq ($(UNAME_S),Darwin)
PLATFORM := darwin
LANCEDB_PRIMARY_LIB := $(RUNTIME_HOME)/lib/$(PLATFORM)_$(ARCH)/liblancedb_go.dylib
LANCEDB_FALLBACK_LIB := $(RUNTIME_HOME)/lib/$(PLATFORM)_$(ARCH)/liblancedb_go.a
LANCEDB_EXTRA_LDFLAGS := -framework Security -framework CoreFoundation
else ifeq ($(UNAME_S),Linux)
PLATFORM := linux
LANCEDB_PRIMARY_LIB := $(RUNTIME_HOME)/lib/$(PLATFORM)_$(ARCH)/liblancedb_go.so
LANCEDB_FALLBACK_LIB := $(RUNTIME_HOME)/lib/$(PLATFORM)_$(ARCH)/liblancedb_go.a
LANCEDB_EXTRA_LDFLAGS := -ldl -lm -lpthread
else ifneq (,$(findstring MINGW,$(UNAME_S)))
PLATFORM := windows
LANCEDB_PRIMARY_LIB := $(RUNTIME_HOME)/lib/$(PLATFORM)_$(ARCH)/liblancedb_go.a
LANCEDB_FALLBACK_LIB := $(RUNTIME_HOME)/lib/$(PLATFORM)_$(ARCH)/liblancedb_go.a
else ifneq (,$(findstring MSYS,$(UNAME_S)))
PLATFORM := windows
LANCEDB_PRIMARY_LIB := $(RUNTIME_HOME)/lib/$(PLATFORM)_$(ARCH)/liblancedb_go.a
LANCEDB_FALLBACK_LIB := $(RUNTIME_HOME)/lib/$(PLATFORM)_$(ARCH)/liblancedb_go.a
else ifneq (,$(findstring CYGWIN,$(UNAME_S)))
PLATFORM := windows
LANCEDB_PRIMARY_LIB := $(RUNTIME_HOME)/lib/$(PLATFORM)_$(ARCH)/liblancedb_go.a
LANCEDB_FALLBACK_LIB := $(RUNTIME_HOME)/lib/$(PLATFORM)_$(ARCH)/liblancedb_go.a
endif

ifneq ($(wildcard $(LANCEDB_PRIMARY_LIB)),)
LANCEDB_LIB_FILE := $(LANCEDB_PRIMARY_LIB)
else ifneq ($(wildcard $(LANCEDB_FALLBACK_LIB)),)
LANCEDB_LIB_FILE := $(LANCEDB_FALLBACK_LIB)
else
LANCEDB_LIB_FILE := $(LANCEDB_PRIMARY_LIB)
endif

LANCEDB_CGO_CFLAGS := -I$(RUNTIME_HOME)/include
LANCEDB_CGO_LDFLAGS := $(LANCEDB_LIB_FILE) $(LANCEDB_EXTRA_LDFLAGS)

build:
	go build -o $(BINARY) ./cmd/echo-fade-memory

setup-lancedb:
	go run ./cmd/setup-lancedb

setup-lancedb-static:
	go run ./cmd/setup-lancedb --static

build-lancedb: setup-lancedb-static
	CGO_CFLAGS="$(LANCEDB_CGO_CFLAGS)" CGO_LDFLAGS="$(LANCEDB_CGO_LDFLAGS)" go build -tags lancedb -o $(BINARY) ./cmd/echo-fade-memory

run: build
	./$(BINARY) $(ARGS)

remember: build
	./$(BINARY) remember "$(CONTENT)"

recall: build
	./$(BINARY) recall "$(QUERY)"

decay: build
	./$(BINARY) decay

test:
	go test ./...

test-lancedb: setup-lancedb-static
	CGO_CFLAGS="$(LANCEDB_CGO_CFLAGS)" CGO_LDFLAGS="$(LANCEDB_CGO_LDFLAGS)" go test -tags lancedb ./...

print-runtime-paths:
	@echo "runtime home: $(RUNTIME_HOME)"
	@echo "platform: $(PLATFORM)_$(ARCH)"
	@echo "data path default: $$HOME/.echo-fade-memory/workspaces/<workspace>/data"
	@echo "lancedb include: $(RUNTIME_HOME)/include"
	@echo "lancedb library: $(LANCEDB_LIB_FILE)"

clean:
	rm -f $(BINARY)
	rm -rf data/

# Docker: single image, runtime switches backend via VECTOR_STORE_TYPE
docker-build:
	docker build -t echo-fade-memory:latest .

# Cross-compile for releases
build-all:
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY)-darwin-arm64 ./cmd/echo-fade-memory
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY)-darwin-amd64 ./cmd/echo-fade-memory
	GOOS=linux GOARCH=amd64 go build -o $(BINARY)-linux-amd64 ./cmd/echo-fade-memory
	GOOS=windows GOARCH=amd64 go build -o $(BINARY)-windows-amd64.exe ./cmd/echo-fade-memory
