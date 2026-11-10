.PHONY: build run test check clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

build: export CGO_ENABLED = 0
build:
	go build -trimpath -ldflags='$(LDFLAGS)' -o lugotui ./cmd/lugotui

run: export CGO_ENABLED = 0
run:
	go run -ldflags='$(LDFLAGS)' ./cmd/lugotui --app internal/assets/demo.lua

test:
	test -z "$$(gofmt -l .)"
	go test -race ./...
	go vet ./...

check: build
	./lugotui --check

clean:
	rm -f -- lugotui
