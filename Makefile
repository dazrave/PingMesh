BINARY := pingmesh
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)"

.PHONY: build test clean cross-compile install lint fmt

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/pingmesh

test:
	go test -race -cover ./...

test-verbose:
	go test -race -cover -v ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .
	goimports -w .

clean:
	rm -f $(BINARY)
	rm -rf dist/

cross-compile: clean
	mkdir -p dist
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 ./cmd/pingmesh
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 ./cmd/pingmesh

install: build
	sudo cp $(BINARY) /usr/local/bin/$(BINARY)
	sudo chmod 755 /usr/local/bin/$(BINARY)

docker:
	docker build -t pingmesh:$(VERSION) .
