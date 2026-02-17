.PHONY: help all build build-dev build-all package release install uninstall run test test-v test-coverage fmt vet lint check clean version doctor

BINARY := highclaw
CMD := ./cmd/highclaw
DIST_DIR := dist
RELEASE_DIR := $(DIST_DIR)/release
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE) -X main.GitCommit=$(GIT_COMMIT)"

UNAME_S := $(shell uname -s 2>/dev/null || echo unknown)
ifeq ($(UNAME_S),Darwin)
	HOST_OS := darwin
else ifeq ($(UNAME_S),Linux)
	HOST_OS := linux
else
	HOST_OS := windows
endif

HOST_ARCH := $(shell go env GOARCH)
ifeq ($(HOST_OS),windows)
	EXE := .exe
else
	EXE :=
endif

ifeq ($(strip $(GOBIN)),)
	ifeq ($(strip $(GOPATH)),)
		INSTALL_DIR := $(HOME)/go/bin
	else
		INSTALL_DIR := $(GOPATH)/bin
	endif
else
	INSTALL_DIR := $(GOBIN)
endif

all: build

help:
	@echo "HighClaw Make targets:"
	@echo "  make build          Build release binary to $(DIST_DIR)/$(BINARY)"
	@echo "  make build-dev      Build debug binary to $(DIST_DIR)/$(BINARY)-dev"
	@echo "  make build-all      Cross-build for macOS/Linux/Windows (amd64+arm64)"
	@echo "  make package        Create tar.gz/zip artifacts in $(RELEASE_DIR)"
	@echo "  make release        build-all + package"
	@echo "  make install        Install binary to $(INSTALL_DIR)"
	@echo "  make uninstall      Remove installed binary from $(INSTALL_DIR)"
	@echo "  make test           Run tests with race detector"
	@echo "  make check          fmt + vet + test"
	@echo "  make clean          Remove build artifacts"

$(DIST_DIR):
	@mkdir -p $(DIST_DIR)

$(RELEASE_DIR):
	@mkdir -p $(RELEASE_DIR)

build: $(DIST_DIR)
	@echo "Building $(BINARY) $(VERSION)..."
	go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)$(EXE) $(CMD)
	@echo "Built $(DIST_DIR)/$(BINARY)$(EXE)"

build-dev: $(DIST_DIR)
	go build -o $(DIST_DIR)/$(BINARY)-dev$(EXE) $(CMD)

run: build
	./$(DIST_DIR)/$(BINARY)$(EXE) gateway --verbose

test:
	go test -race ./...

test-v:
	go test -race -v ./...

test-coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

vet:
	go vet ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 || (echo "Install golangci-lint first: https://golangci-lint.run" && exit 1)
	golangci-lint run

check: vet test

clean:
	rm -rf $(DIST_DIR) coverage.out coverage.html

install: build
	@mkdir -p "$(INSTALL_DIR)"
	cp "$(DIST_DIR)/$(BINARY)$(EXE)" "$(INSTALL_DIR)/$(BINARY)$(EXE)"
	@chmod +x "$(INSTALL_DIR)/$(BINARY)$(EXE)" 2>/dev/null || true
	@echo "Installed to $(INSTALL_DIR)/$(BINARY)$(EXE)"

uninstall:
	rm -f "$(INSTALL_DIR)/$(BINARY)" "$(INSTALL_DIR)/$(BINARY).exe"
	@echo "Removed $(INSTALL_DIR)/$(BINARY)"

build-all: $(DIST_DIR)
	@set -e; \
	for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*}; \
		GOARCH=$${platform#*/}; \
		ext=""; \
		if [ "$$GOOS" = "windows" ]; then ext=".exe"; fi; \
		out="$(DIST_DIR)/$(BINARY)-$$GOOS-$$GOARCH$$ext"; \
		echo "Building $$out"; \
		CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH go build $(LDFLAGS) -o "$$out" $(CMD); \
	done

package: build-all $(RELEASE_DIR)
	@set -e; \
	extras="README.md"; \
	if [ -f "$(CURDIR)/LICENSE" ]; then extras="$$extras LICENSE"; fi; \
	for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*}; \
		GOARCH=$${platform#*/}; \
		ext=""; \
		if [ "$$GOOS" = "windows" ]; then ext=".exe"; fi; \
		bin="$(DIST_DIR)/$(BINARY)-$$GOOS-$$GOARCH$$ext"; \
		name="$(BINARY)-$(VERSION)-$$GOOS-$$GOARCH"; \
		if [ "$$GOOS" = "windows" ]; then \
			if command -v zip >/dev/null 2>&1; then \
				zip -j "$(RELEASE_DIR)/$$name.zip" "$$bin" $$extras; \
			else \
				echo "zip not found, skipping $$name.zip"; \
			fi; \
		else \
			tar -czf "$(RELEASE_DIR)/$$name.tar.gz" -C "$(CURDIR)/$(DIST_DIR)" "$$(basename "$$bin")" -C "$(CURDIR)" $$extras; \
		fi; \
	done
	@echo "Artifacts in $(RELEASE_DIR)"

release: package

doctor:
	@echo "version=$(VERSION)"
	@echo "go=$$(go version)"
	@echo "host=$(HOST_OS)/$(HOST_ARCH)"
	@echo "install_dir=$(INSTALL_DIR)"

version:
	@echo "$(VERSION)"
