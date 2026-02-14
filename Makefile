.PHONY: build run test clean dev install

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE) -X main.GitCommit=$(GIT_COMMIT)"

# Output
BINARY := highclaw
DIST_DIR := dist

# Default target
all: build

# Build the binary
build:
	@echo "🦀 Building HighClaw $(VERSION)..."
	go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY) ./cmd/highclaw
	@echo "✅ Built $(DIST_DIR)/$(BINARY)"

# Run in development mode
run: build
	./$(DIST_DIR)/$(BINARY) gateway --verbose

# Run development mode with hot reload (requires air)
dev:
	@which air > /dev/null 2>&1 || (echo "Install air: go install github.com/air-verse/air@latest" && exit 1)
	air -c .air.toml

# Run all tests
test:
	go test -race -cover ./...

# Run tests with verbose output
test-v:
	go test -race -cover -v ./...

# Run tests with coverage report
test-coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "📊 Coverage report: coverage.html"

# Lint (requires golangci-lint)
lint:
	@which golangci-lint > /dev/null 2>&1 || (echo "Install: brew install golangci-lint" && exit 1)
	golangci-lint run

# Clean build artifacts
clean:
	rm -rf $(DIST_DIR) coverage.out coverage.html

# Install globally
install: build
	cp $(DIST_DIR)/$(BINARY) $(GOPATH)/bin/$(BINARY) 2>/dev/null || \
	  cp $(DIST_DIR)/$(BINARY) /usr/local/bin/$(BINARY)
	@echo "✅ Installed $(BINARY)"

# Cross-compilation
build-all: build-linux build-darwin build-windows

build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-linux-amd64 ./cmd/highclaw
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-linux-arm64 ./cmd/highclaw

build-darwin:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-darwin-amd64 ./cmd/highclaw
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-darwin-arm64 ./cmd/highclaw

build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-windows-amd64.exe ./cmd/highclaw

# Docker
docker-build:
	docker build -t highclaw:$(VERSION) .

# Print version
version:
	@echo "$(VERSION)"
