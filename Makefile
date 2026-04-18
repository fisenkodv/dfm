.PHONY: build test race coverage lint tidy clean

BIN := dfm
PKG := github.com/fisenkodv/dfm
REV := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.revision=$(REV)

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN) .

test:
	go test ./...

race:
	go test -race ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

clean:
	rm -f $(BIN) coverage.out
	rm -rf dist/
