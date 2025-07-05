#!/bin/bash

# Release script for NextDNS IP Updater
# This script helps prepare a release by checking versions and providing instructions

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}NextDNS IP Updater Release Helper${NC}"
echo "======================================"

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo -e "${RED}Error: Not in a git repository${NC}"
    exit 1
fi

# Get current version from pyproject.toml
CURRENT_VERSION=$(grep "version = " pyproject.toml | sed -e 's/version = "\(.*\)"/\1/g')
echo -e "Current version in pyproject.toml: ${YELLOW}${CURRENT_VERSION}${NC}"

# Check if there are uncommitted changes
if ! git diff-index --quiet HEAD --; then
    echo -e "${YELLOW}Warning: You have uncommitted changes${NC}"
    git status --short
    echo ""
fi

# Show latest git tag
LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "No tags found")
echo -e "Latest git tag: ${YELLOW}${LATEST_TAG}${NC}"

# Show what would be built
echo ""
echo -e "${GREEN}This release would build:${NC}"
echo "- Docker image: ghcr.io/nilbot/update-local-wan-ip-nextdns:${CURRENT_VERSION}"
echo "- Go binary: nextdns-ip-updater-linux-amd64"
echo "- Go binary: nextdns-ip-updater-darwin-arm64"

# Check if Go binaries can be built
echo ""
echo -e "${GREEN}Pre-release checks:${NC}"
if command -v go >/dev/null 2>&1; then
    echo "✓ Go is installed: $(go version)"
    if make dev-test >/dev/null 2>&1; then
        echo "✓ Go build and tests pass"
    else
        echo -e "${YELLOW}⚠ Go build or tests failed - run 'make dev-test' for details${NC}"
    fi
else
    echo -e "${YELLOW}⚠ Go is not installed${NC}"
fi

# Check if Docker can build
if command -v docker >/dev/null 2>&1; then
    echo "✓ Docker is available"
else
    echo -e "${YELLOW}⚠ Docker is not available${NC}"
fi

echo ""
echo -e "${GREEN}To create a release:${NC}"
echo "1. Make sure all changes are committed and pushed"
echo "2. Create and push a tag:"
echo -e "   ${YELLOW}git tag v${CURRENT_VERSION}${NC}"
echo -e "   ${YELLOW}git push origin v${CURRENT_VERSION}${NC}"
echo "3. Go to GitHub and create a release from the tag"
echo "4. GitHub Actions will automatically:"
echo "   - Build and push the Docker image"
echo "   - Build and attach Go binaries to the release"

echo ""
echo -e "${GREEN}Manual testing commands:${NC}"
echo "Build and test Go version:"
echo -e "   ${YELLOW}make build-all${NC}"
echo "Build and test Docker version:"
echo -e "   ${YELLOW}docker-compose build${NC}"

echo ""
echo -e "${GREEN}Version consistency check:${NC}"
if [[ "${LATEST_TAG}" == "v${CURRENT_VERSION}" ]]; then
    echo -e "${YELLOW}Warning: Tag v${CURRENT_VERSION} already exists${NC}"
elif [[ "${LATEST_TAG}" == "${CURRENT_VERSION}" ]]; then
    echo -e "${YELLOW}Warning: Tag ${CURRENT_VERSION} already exists${NC}"
else
    echo -e "${GREEN}✓ Version ${CURRENT_VERSION} is ready for release${NC}"
fi
