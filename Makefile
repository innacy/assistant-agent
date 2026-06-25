.PHONY: build run-serve run-daemon run-auth test clean web

BINARY=bin/assistant-agent

web:
	cd web && npm run build
	rm -rf pkg/api/web_dist
	cp -r web/dist pkg/api/web_dist

build: web
	go build -o $(BINARY) main.go

build-go:
	go build -o $(BINARY) main.go

run-serve: build
	./$(BINARY) --serve

run-daemon: build
	./$(BINARY) --daemon

run-auth: build-go
	./$(BINARY) --auth

test:
	go test ./... -v

clean:
	rm -rf bin/ pkg/api/web_dist
