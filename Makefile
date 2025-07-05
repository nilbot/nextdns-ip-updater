# Makefile for building NextDNS IP Updater Go version

# Build info
BINARY_NAME=nextdns-ip-updater
VERSION?=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-ldflags "-w -s -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}"

# Default target
.PHONY: all
all: build

# Build for current platform
.PHONY: build
build:
	go build ${LDFLAGS} -o ${BINARY_NAME} main.go

# Build for all target platforms
.PHONY: build-all
build-all: build-linux-amd64 build-darwin-arm64

# Build for Linux x64
.PHONY: build-linux-amd64
build-linux-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ${LDFLAGS} -o ${BINARY_NAME}-linux-amd64 main.go

# Build for macOS ARM64
.PHONY: build-darwin-arm64
build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ${LDFLAGS} -o ${BINARY_NAME}-darwin-arm64 main.go

# Clean build artifacts
.PHONY: clean
clean:
	rm -f ${BINARY_NAME} ${BINARY_NAME}-*

# Run tests
.PHONY: test
test:
	go test -v ./...

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Vet code
.PHONY: vet
vet:
	go vet ./...

# Run linter (requires golangci-lint)
.PHONY: lint
lint:
	golangci-lint run

# Install dependencies
.PHONY: deps
deps:
	go mod download
	go mod tidy

# Create release artifacts (for local testing)
.PHONY: release-artifacts
release-artifacts: build-all
	@echo "Creating release artifacts..."
	@for binary in ${BINARY_NAME}-linux-amd64 ${BINARY_NAME}-darwin-arm64; do \
		if [ -f $$binary ]; then \
			echo "Creating archive for $$binary"; \
			tar -czf $$binary.tar.gz $$binary; \
			sha256sum $$binary > $$binary.sha256; \
			sha256sum $$binary.tar.gz > $$binary.tar.gz.sha256; \
		fi \
	done
	@echo "Release artifacts created:"
	@ls -la ${BINARY_NAME}-* | grep -E '\.(tar\.gz|sha256)$$' || true

# Quick development test
.PHONY: dev-test
dev-test: fmt vet test build
	@echo "Development tests passed! Binary built successfully."

# Watch for changes and rebuild (requires entr)
.PHONY: watch
watch:
	@command -v entr >/dev/null 2>&1 || { echo "Install 'entr' to use watch mode: brew install entr"; exit 1; }
	@echo "Watching for changes... (requires 'entr' installed)"
	@find . -name "*.go" | entr -c make dev-test

# Check if required tools are installed
.PHONY: check-tools
check-tools:
	@echo "Checking for required tools..."
	@command -v go >/dev/null 2>&1 || { echo "Go is not installed"; exit 1; }
	@command -v git >/dev/null 2>&1 || { echo "Git is not installed"; exit 1; }
	@go version
	@git --version
	@echo "All required tools are available"

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build              - Build for current platform"
	@echo "  build-all          - Build for all target platforms"
	@echo "  build-linux-amd64  - Build for Linux x64"
	@echo "  build-darwin-arm64 - Build for macOS ARM64"
	@echo "  clean              - Clean build artifacts"
	@echo "  test               - Run tests"
	@echo "  fmt                - Format code"
	@echo "  vet                - Vet code"
	@echo "  lint               - Run linter"
	@echo "  deps               - Install dependencies"
	@echo "  release-artifacts  - Create release artifacts (tar.gz + checksums)"
	@echo "  dev-test           - Quick development test (fmt + vet + test + build)"
	@echo "  watch              - Watch for changes and rebuild (requires entr)"
	@echo "  check-tools        - Check if required tools are installed"
	@echo "  help               - Show this help"
