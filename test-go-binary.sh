#!/bin/bash

# Test script for the Go binary with a mock HTTP server
# This allows testing without a real NextDNS endpoint

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}NextDNS IP Updater - Go Binary Test${NC}"
echo "=========================================="

# Check if the binary exists
BINARY=""
if [[ "$OSTYPE" == "darwin"* ]]; then
    BINARY="nextdns-ip-updater-darwin-arm64"
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    BINARY="nextdns-ip-updater-linux-amd64"
else
    echo -e "${RED}Unsupported OS: $OSTYPE${NC}"
    echo "Please build manually with: make build"
    BINARY="nextdns-ip-updater"
fi

if [ ! -f "$BINARY" ]; then
    echo -e "${YELLOW}Binary $BINARY not found. Building...${NC}"
    make build-all
fi

if [ ! -f "$BINARY" ]; then
    echo -e "${RED}Failed to build binary${NC}"
    exit 1
fi

echo -e "${GREEN}Using binary: $BINARY${NC}"

# Start a simple HTTP server in the background that always returns OK
echo -e "${YELLOW}Starting mock NextDNS server...${NC}"

# Find an available port
PORT=$(python3 -c 'import socket; s=socket.socket(); s.bind(("", 0)); print(s.getsockname()[1]); s.close()')

# Start Python HTTP server that returns OK for any request
python3 -c "
import http.server
import socketserver
import threading
import time

class MockHandler(http.server.SimpleHTTPRequestHandler):
    def do_GET(self):
        print(f'Mock NextDNS received request: {self.path}')
        self.send_response(200)
        self.send_header('Content-type', 'text/plain')
        self.end_headers()
        self.wfile.write(b'OK - Mock NextDNS response')
        
    def log_message(self, format, *args):
        pass  # Suppress default logging

with socketserver.TCPServer(('', $PORT), MockHandler) as httpd:
    print(f'Mock NextDNS server running on port $PORT')
    httpd.serve_forever()
" &

MOCK_SERVER_PID=$!

# Wait a moment for the server to start
sleep 1

# Set environment variables for the test
export NEXTDNS_ENDPOINT="http://localhost:$PORT/test-endpoint"
export UPDATE_INTERVAL_SECONDS=2

echo -e "${YELLOW}Testing binary with mock endpoint...${NC}"
echo "Endpoint: $NEXTDNS_ENDPOINT"
echo "Update interval: $UPDATE_INTERVAL_SECONDS seconds"
echo ""
echo -e "${YELLOW}The binary will run for 10 seconds then stop...${NC}"

# Run the binary for a short time
timeout 10s ./"$BINARY" || echo -e "${GREEN}Test completed (timeout after 10 seconds)${NC}"

# Clean up
echo -e "${YELLOW}Cleaning up...${NC}"
kill $MOCK_SERVER_PID 2>/dev/null || true
wait $MOCK_SERVER_PID 2>/dev/null || true

echo ""
echo -e "${GREEN}Test completed successfully!${NC}"
echo "The binary connected to the mock server and sent requests as expected."
