# Developer Setup Guide

This guide helps you set up the development environment for the NextDNS IP Updater project.

## Prerequisites

### Required Tools
- **Go 1.23+**: [Install Go](https://golang.org/doc/install)
- **Git**: [Install Git](https://git-scm.com/downloads)
- **Make**: Usually pre-installed on macOS/Linux

### Optional Tools
- **Docker**: For testing the Python version and container builds
- **golangci-lint**: For comprehensive Go linting
- **entr**: For file watching during development (`brew install entr` on macOS)

## Quick Start

1. **Clone and setup**:
   ```bash
   git clone <repository>
   cd update-local-wan-ip-nextdns
   make check-tools  # Verify required tools
   make deps         # Download Go dependencies
   ```

2. **Run tests**:
   ```bash
   make dev-test     # Quick development test (format + vet + test + build)
   ```

3. **Build binaries**:
   ```bash
   make build-all    # Build for all platforms (Linux x64, macOS ARM64)
   ```

## Development Workflow

### Quick Development Loop
```bash
# Make changes to code
make dev-test                    # Run tests and build
# OR for continuous development:
make watch                       # Auto-rebuild on file changes (requires entr)
```

### Testing
```bash
make test                        # Run unit tests
go test -v                       # Verbose test output
go test -race -coverprofile=coverage.out ./...  # With race detection and coverage
```

### Building
```bash
make build                       # Build for current platform
make build-all                   # Build for all target platforms
make release-artifacts           # Create release-ready artifacts (binaries + checksums)
```

### Code Quality
```bash
make fmt                         # Format code
make vet                         # Run go vet
make lint                        # Run golangci-lint (if installed)
```

## Testing the Application

### Unit Tests
```bash
make test
```

### Manual Testing
```bash
# Set environment variables
export NEXTDNS_ENDPOINT="https://link-ip.nextdns.io/YOUR_ID/YOUR_EXT_ID"
export UPDATE_INTERVAL_SECONDS=60

# Run the binary
./nextdns-ip-updater-darwin-arm64  # On macOS
./nextdns-ip-updater-linux-amd64   # On Linux
```

### Docker Testing (Python version)
```bash
docker-compose build
docker-compose up
```

## Release Process

1. **Prepare release**:
   ```bash
   ./release.sh                   # Check release readiness
   make dev-test                  # Final validation
   ```

2. **Update version** (if needed):
   - Edit `pyproject.toml` version field
   - Commit changes

3. **Create and push tag**:
   ```bash
   git tag v0.1.2                 # Use semantic versioning
   git push origin v0.1.2
   ```

4. **Create GitHub release**:
   - Go to GitHub repository
   - Create a new release from the tag
   - GitHub Actions will automatically:
     - Build and publish Docker image
     - Build Go binaries for Linux x64 and macOS ARM64
     - Attach binaries, archives, and checksums to the release

## Project Structure

```
├── main.go                      # Go implementation
├── main_test.go                 # Go unit tests
├── main.py                      # Python implementation
├── go.mod, go.sum               # Go dependencies
├── pyproject.toml               # Python project config
├── Makefile                     # Build automation
├── Dockerfile                   # Python container
├── docker-compose.yml           # Local development
├── release.sh                   # Release helper script
├── .github/workflows/
│   ├── release.yml             # Docker image CI/CD
│   ├── release-go.yml          # Go binaries CI/CD
│   └── go-tests.yml            # Go testing CI
├── deploy/                      # Kubernetes manifests (Python)
├── README.md                    # Python version docs
└── README-go.md                # Go version docs
```

## Available Make Targets

Run `make help` to see all available targets:

- `make build` - Build for current platform
- `make build-all` - Build for all target platforms  
- `make test` - Run tests
- `make dev-test` - Quick development test (fmt + vet + test + build)
- `make clean` - Clean build artifacts
- `make release-artifacts` - Create release artifacts
- `make watch` - Watch for changes and rebuild (requires entr)
- `make check-tools` - Check if required tools are installed

## Troubleshooting

### Build Issues
- Ensure Go 1.23+ is installed: `go version`
- Clear module cache: `go clean -modcache`
- Re-download dependencies: `make deps`

### Test Issues
- Tests may take ~35 seconds due to timeout testing
- If tests hang, check for network connectivity issues
- Run with verbose output: `go test -v`

### Environment Variables
- `NEXTDNS_ENDPOINT` (required): Your NextDNS endpoint URL
- `UPDATE_INTERVAL_SECONDS` (optional): Update interval in seconds (default: 300)

## GitHub Actions

The project has three GitHub Actions workflows:

1. **Go Tests** (`.github/workflows/go-tests.yml`):
   - Runs on every push/PR
   - Tests code formatting, linting, and unit tests
   - Tests cross-compilation

2. **Docker Release** (`.github/workflows/release.yml`):
   - Runs on GitHub releases
   - Builds and publishes Docker image for Python version

3. **Go Release** (`.github/workflows/release-go.yml`):
   - Runs on GitHub releases  
   - Builds Go binaries for Linux x64 and macOS ARM64
   - Creates checksums and archives
   - Attaches artifacts to GitHub release

All workflows use proper versioning, security practices, and artifact management.
