.PHONY: build run store recall decay clean

BINARY := echo-fade-memory

build:
	go build -o $(BINARY) ./cmd/echo-fade-memory

run: build
	./$(BINARY) $(ARGS)

store: build
	./$(BINARY) store "$(CONTENT)"

recall: build
	./$(BINARY) recall "$(QUERY)"

decay: build
	./$(BINARY) decay

clean:
	rm -f $(BINARY)
	rm -rf data/

# Cross-compile for releases
build-all:
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY)-darwin-arm64 ./cmd/echo-fade-memory
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY)-darwin-amd64 ./cmd/echo-fade-memory
	GOOS=linux GOARCH=amd64 go build -o $(BINARY)-linux-amd64 ./cmd/echo-fade-memory
	GOOS=windows GOARCH=amd64 go build -o $(BINARY)-windows-amd64.exe ./cmd/echo-fade-memory
