.PHONY: build run remember recall decay test clean docker-build build-all

BINARY := echo-fade-memory

build:
	go build -o $(BINARY) ./cmd/echo-fade-memory

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
