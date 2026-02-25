VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"
WAILS   := $(shell go env GOBIN)/wails

.PHONY: dev build test clean

dev:
	$(WAILS) dev

build:
	$(WAILS) build $(LDFLAGS)

test:
	go test -race -count=1 ./...

clean:
	rm -rf build/bin
	rm -rf frontend/dist
	rm -rf frontend/node_modules
