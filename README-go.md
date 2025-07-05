# NextDNS IP Updater - Go Version

This is the Go implementation of the NextDNS IP Updater, providing static binaries for easy deployment without Docker.

## Features

- Static binaries with no external dependencies
- Cross-platform support (Linux x64, macOS ARM64)
- Same functionality as the Python version
- Structured JSON logging
- Configurable update intervals
- Automatic builds via GitHub Actions

## Download

Download the latest pre-built binaries from the [GitHub Releases](https://github.com/nilbot/update-local-wan-ip-nextdns/releases) page.

Available binaries:
- `nextdns-ip-updater-linux-amd64` - Linux x64 static binary
- `nextdns-ip-updater-darwin-arm64` - macOS ARM64 static binary

Each release includes:
- Raw binaries
- Compressed tar.gz archives
- SHA256 checksums for verification

## Installation

1. Download the appropriate binary for your platform from the releases page
2. Make it executable:
   ```bash
   chmod +x nextdns-ip-updater-linux-amd64  # or nextdns-ip-updater-darwin-arm64
   ```
3. Optionally, move it to a directory in your PATH:
   ```bash
   sudo mv nextdns-ip-updater-linux-amd64 /usr/local/bin/nextdns-ip-updater
   ```

## Configuration

Configuration is done through environment variables:

- `NEXTDNS_ENDPOINT`: The NextDNS endpoint URL to update (required)
  - Format: `https://link-ip.nextdns.io/YOUR_ID/YOUR_EXT_ID`
- `UPDATE_INTERVAL_SECONDS`: Time between updates in seconds (default: 300 seconds / 5 minutes)

## Usage

### Direct execution

```bash
export NEXTDNS_ENDPOINT="https://link-ip.nextdns.io/YOUR_ID/YOUR_EXT_ID"
export UPDATE_INTERVAL_SECONDS=300
./nextdns-ip-updater-linux-amd64
```

### With systemd (Linux)

Create a systemd service file `/etc/systemd/system/nextdns-ip-updater.service`:

```ini
[Unit]
Description=NextDNS IP Updater
After=network.target

[Service]
Type=simple
User=nobody
Group=nobody
Environment=NEXTDNS_ENDPOINT=https://link-ip.nextdns.io/YOUR_ID/YOUR_EXT_ID
Environment=UPDATE_INTERVAL_SECONDS=300
ExecStart=/usr/local/bin/nextdns-ip-updater
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Then enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable nextdns-ip-updater
sudo systemctl start nextdns-ip-updater
```

### With launchd (macOS)

Create a plist file `~/Library/LaunchAgents/com.nextdns.ip-updater.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.nextdns.ip-updater</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/nextdns-ip-updater</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>NEXTDNS_ENDPOINT</key>
        <string>https://link-ip.nextdns.io/YOUR_ID/YOUR_EXT_ID</string>
        <key>UPDATE_INTERVAL_SECONDS</key>
        <string>300</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/nextdns-ip-updater.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/nextdns-ip-updater.log</string>
</dict>
</plist>
```

Then load the service:

```bash
launchctl load ~/Library/LaunchAgents/com.nextdns.ip-updater.plist
launchctl start com.nextdns.ip-updater
```

## Building from Source

If you want to build the binaries yourself:

1. Install Go 1.23 or later
2. Clone this repository
3. Build for your platform:
   ```bash
   make build
   ```
4. Or build for all supported platforms:
   ```bash
   make build-all
   ```

### Development

For development work, see [DEVELOPMENT.md](DEVELOPMENT.md) for a comprehensive guide.

Quick development commands:
```bash
make dev-test           # Format, vet, test, and build
make test               # Run unit tests
make release-artifacts  # Create release-ready artifacts
./test-go-binary.sh     # Test with mock server
```

## Log Output

The application outputs structured JSON logs to stdout. Example:

```json
{"level":"info","msg":"Starting NextDNS IP updater","endpoint":"https://link-ip.nextdns.io/xxx/yyy","interval_seconds":300,"version":"v0.1.1","build_time":"2025-07-05T16:42:52Z","time":"2025-07-05T16:42:52Z"}
{"level":"info","msg":"Successfully updated NextDNS","endpoint":"https://link-ip.nextdns.io/xxx/yyy","time":"2025-07-05T16:42:52Z"}
{"level":"info","msg":"Update cycle completed","success":true,"time":"2025-07-05T16:42:52Z"}
```

## Verification

To verify the integrity of downloaded binaries, use the provided SHA256 checksums:

```bash
# Download the binary and its checksum
wget https://github.com/nilbot/update-local-wan-ip-nextdns/releases/download/v0.1.1/nextdns-ip-updater-linux-amd64
wget https://github.com/nilbot/update-local-wan-ip-nextdns/releases/download/v0.1.1/nextdns-ip-updater-linux-amd64.sha256

# Verify the checksum
sha256sum -c nextdns-ip-updater-linux-amd64.sha256
```
