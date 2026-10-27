.PHONY: build run test check clean

build: export CGO_ENABLED = 0
build:
	go build -trimpath -ldflags='-s -w' -o lugotui ./cmd/lugotui

run:
	go run ./cmd/lugotui

test:
	go test -race ./...
	go vet ./...

check: build
	./lugotui --check

clean:
	rm -- lugotui
