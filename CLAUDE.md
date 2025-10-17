# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a NextDNS IP updater service with **dual implementations**:
- **Python version**: Docker-based deployment (primary README.md) - **Deprecated, use Go version**
- **Go version**: Docker images (multi-arch) + static binaries for systemd/launchd (README-go.md)

Both implementations provide identical functionality: periodically calling a NextDNS endpoint to update the WAN IP address.

**Recommended:** Use the Go version for production deployments (smaller image size, better performance, multi-arch support).

## Build Commands

### Go Version (Primary Development)

**Binaries:**
```bash
make build              # Build for current platform
make build-all          # Build for Linux x64/ARM64 and macOS ARM64
make test               # Run Go unit tests
make dev-test           # Format + vet + test + build (recommended during development)
make fmt                # Format Go code
make vet                # Run go vet
make lint               # Run golangci-lint (if installed)
make clean              # Remove build artifacts
make release-artifacts  # Create release-ready tar.gz + checksums
```

**Docker (Go):**
```bash
# Build multi-arch Go Docker image locally (requires buildx)
docker buildx build --platform linux/amd64,linux/arm64 \
  -f Dockerfile-go \
  --build-arg VERSION=$(git describe --tags --always) \
  --build-arg BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  -t nextdns-ip-updater:go .

# Build for specific platform
docker buildx build --platform linux/amd64 -f Dockerfile-go -t nextdns-ip-updater:go .
```

### Python Version (Docker)

```bash
docker-compose up -d    # Build and run Python version
docker-compose build    # Build Python container
docker logs -f nextdns-ip-updater  # View logs
```

## Architecture & Code Structure

### Dual Implementation Pattern

Both `main.go` and `main.py` implement the same core logic:
1. Read `NEXTDNS_ENDPOINT` and `UPDATE_INTERVAL_SECONDS` from environment
2. Validate the NextDNS endpoint URL
3. Run an infinite loop that calls the endpoint via HTTP GET
4. Log results using structured JSON logging (logrus for Go, structlog for Python)
5. Sleep for the configured interval

The Go version adds:
- Graceful shutdown via SIGINT/SIGTERM signal handling
- Build-time version injection via LDFLAGS
- Static binary compilation (CGO_ENABLED=0)

### Version Management

Version is tracked in `pyproject.toml` (currently 0.1.4). For Go binaries, version is injected at build time:

```makefile
VERSION?=$(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-w -s -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}"
```

When creating releases, update `pyproject.toml` version, then tag with `git tag vX.Y.Z`.

### Testing Philosophy

Go tests in `main_test.go` are designed for fast execution:
- Timeout tests use context cancellation to complete in ~100ms
- No actual network delays during testing
- Tests should complete in under 1 second total

Run tests frequently during development: `make dev-test`

## Release Workflow

### Automated GitHub Actions

1. **go-tests.yml**: Runs on all pushes/PRs
   - Validates formatting, linting, tests
   - Ensures cross-compilation works

2. **release-go.yml**: Runs on GitHub releases (tags)
   - Builds Go binaries for Linux x64/ARM64 and macOS ARM64
   - Creates tar.gz archives and SHA256 checksums
   - Attaches artifacts to the GitHub release

3. **release-go-docker.yml**: Runs on GitHub releases (tags) - **PRIMARY DEPLOYMENT**
   - Builds multi-arch Go Docker image (linux/amd64, linux/arm64)
   - Uses distroless base (~10MB vs ~1GB Python)
   - Publishes to `ghcr.io/nilbot/nextdns-ip-updater` with `-go` suffix
   - Tags: `{version}-go`, `{major}.{minor}-go`, `{major}-go`, `latest-go`

4. **release.yml**: Runs on GitHub releases (tags) - **DEPRECATED**
   - Builds and publishes Docker image for Python version
   - Tags: `{version}`, `{major}.{minor}`, `{major}`, `latest`

### Creating a Release

```bash
# 1. Update version in pyproject.toml
# 2. Commit and push changes
git commit -m "bump version to vX.Y.Z"
git push

# 3. Create and push tag
git tag vX.Y.Z
git push origin vX.Y.Z

# 4. Create GitHub release from tag (triggers automated builds)
```

## Configuration

Environment variables:
- `NEXTDNS_ENDPOINT`: (required) NextDNS endpoint URL format: `https://link-ip.nextdns.io/YOUR_ID/YOUR_EXT_ID`
- `UPDATE_INTERVAL_SECONDS`: (optional) Update interval in seconds, default: 300 (5 minutes)

## Deployment Options

### Container Deployments (Recommended)

- **Kubernetes**: See `deploy/` directory - uses Go Docker image `ghcr.io/nilbot/nextdns-ip-updater:{version}-go`
  - Multi-arch support: automatically selects linux/amd64 or linux/arm64
  - ~10MB image vs ~1GB Python image

- **Docker Compose**: Create docker-compose.yml with Go image:
  ```yaml
  services:
    nextdns-updater:
      image: ghcr.io/nilbot/nextdns-ip-updater:latest-go
      restart: unless-stopped
      environment:
        - NEXTDNS_ENDPOINT=https://link-ip.nextdns.io/$NEXTDNS_ID/$NEXTDNS_EXT_ID
        - UPDATE_INTERVAL_SECONDS=300
  ```

### Binary Deployments

- **systemd**: See README-go.md for Linux service configuration
- **launchd**: See README-go.md for macOS service configuration

### Legacy (Deprecated)

- Python Docker image still available: `ghcr.io/nilbot/nextdns-ip-updater:{version}` (no `-go` suffix)

## Common Development Tasks

### Quick Development Loop

```bash
# Make code changes
make dev-test          # Fast feedback: format, vet, test, build
```

### Adding Features

When modifying the core update logic, changes must be made to BOTH:
- `main.go` (Go implementation)
- `main.py` (Python implementation)

Ensure both maintain feature parity and identical logging output structure.

### Testing Locally

```bash
export NEXTDNS_ENDPOINT="https://link-ip.nextdns.io/YOUR_ID/YOUR_EXT_ID"
export UPDATE_INTERVAL_SECONDS=60
./nextdns-ip-updater    # Run built binary
```

Or use the test script:
```bash
./test-go-binary.sh     # Runs with mock server
```
