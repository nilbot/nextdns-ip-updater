# AGENTS.md

This file provides guidance to AI coding assistants (like Antigravity or Claude Code) when working with code in this repository. It prevents context shift and ensures consistency with the current Go-only codebase.

## Project Overview

This is a NextDNS IP updater service written in Go. It runs as a lightweight, static binary or within a minimal distroless Docker container (~10MB) to periodically update a NextDNS endpoint with the public WAN IP of the host network.

## Build & Development Commands

Use the following `make` targets during development:

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

### Docker Builds

Build the multi-arch Go Docker image locally:
```bash
docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg VERSION=$(cat VERSION) \
  --build-arg BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  -t nextdns-ip-updater:latest .
```

## Architecture & Code Structure

The application is structured into the following components:
1. **`main.go`**: Core application loop. Reads environment variables, initializes components, and coordinates updates.
2. **`dns.go`**: Validates NextDNS connectivity and provides startup DNS validation.
3. **`backoff.go`**: Implements exponential backoff retry logic.
4. **`health.go`**: Runs a background HTTP server for health checking and metric collection.

### Key Features
- **Graceful Shutdown**: Listens for SIGINT/SIGTERM and terminates gracefully, shutting down the HTTP health server.
- **DNS Resilience**: If DNS is down at startup, it performs exponential backoff retries until DNS is restored, avoiding crash loops.
- **HTTP Health Endpoints**:
  - `GET /health` - Liveness check (checks uptime, success/error metrics, fails if updates are stuck for > 2x interval).
  - `GET /ready` - Readiness check (returns 200 OK only after the first successful NextDNS update).
  - `GET /metrics` - Prometheus metrics (`nextdns_updates_total`, `nextdns_errors_total`, etc.).

## Version Management

The application version is tracked in a single source of truth:
- **`VERSION` file**: Contains the current semantic version (e.g., `0.1.6`).

During builds, this version is injected via `LDFLAGS`:
```makefile
VERSION?=$(shell cat VERSION)
LDFLAGS=-ldflags "-w -s -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}"
```

## Testing Philosophy

Unit tests are written with high coverage targets (85%+ for key components).

**Test Files:**
- `backoff_test.go` - Tests exponential backoff intervals and ceilings.
- `dns_test.go` - Tests DNS checking and retry mechanisms.
- `health_test.go` - Tests HTTP health handlers, JSON metrics, and Prometheus formats.
- `main_test.go` - Tests core update functionality with a mock HTTP server.

**Run tests during development:**
```bash
make dev-test
```

## Release Workflow

### Automated GitHub Actions
- **`go-tests.yml`**: Runs on all pushes/PRs. Formats, vets, and tests the code.
- **`release-go.yml`**: Runs on Git tags. Compiles Go binaries for Linux x64/ARM64 and macOS ARM64, compresses them, and attaches them to the release.
- **`release-go-docker.yml`**: Runs on Git tags. Builds multi-arch Go Docker images and publishes them to `ghcr.io/nilbot/nextdns-ip-updater`.

### Creating a Release
To publish a new version:
1. Update the version string in the `VERSION` file (e.g., `0.1.7`).
2. Commit and push:
   ```bash
   git commit -am "bump version to v0.1.7"
   git push
   ```
3. Create and push the Git tag:
   ```bash
   git tag v0.1.7
   git push origin v0.1.7
   ```
4. Draft the GitHub release from the tag to trigger automated builds.

## Configuration Reference

The application is configured using environment variables:
- `NEXTDNS_ENDPOINT` (Required): NextDNS link-ip URL, e.g., `https://link-ip.nextdns.io/YOUR_ID/YOUR_EXT_ID`.
- `UPDATE_INTERVAL_SECONDS` (Optional): Update interval in seconds (default: `300`).
- `HEALTH_SERVER_PORT` (Optional): HTTP server port for health checks (default: `8080`, configurable to any port like `48080` for local dev/launchd).
