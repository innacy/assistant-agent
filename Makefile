.PHONY: build run-serve run-daemon run-auth test clean

BINARY=bin/assistant-agent

build:
	go build -o $(BINARY) main.go

run-serve: build
	./$(BINARY) --serve

run-daemon: build
	./$(BINARY) --daemon

run-auth: build
	./$(BINARY) --auth

test:
	go test ./... -v

clean:
	rm -rf bin/
