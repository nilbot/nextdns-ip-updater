# Session Handoff: Setup Launchd Daemon

- **Date**: 2026-06-10 16:51:17
- **Conversation ID**: `6c42c0ea-5b6c-47e7-b6b1-484865a11a68`

## 📌 Project Overview & Handoff Summary

### Original User Request
> I want to have this updater run inside my current Mac, automatic and unattended, using launchctl (to mimic systemd) Please create the necessary config and set up the daemon for me.

## 📋 Proposed Implementation Plan

This plan outlines how to build and configure the NextDNS IP Updater to run as an unattended background service on macOS using `launchctl`.

## User Review Required

> [!IMPORTANT]
> We successfully read the NextDNS endpoint from [01-secrets.yaml](file:///Users/nilbot/devel/nilbot.net/homelab/update-local-wan-ip-nextdns/deploy/01-secrets.yaml):
> `https://link-ip.nextdns.io/YOUR_ID/YOUR_EXT_ID`
>
> Please confirm if you prefer **Option A (User LaunchAgent)** or **Option B (System LaunchDaemon)** to proceed.

## Open Questions

> [!IMPORTANT]
> 1. **Service Type**: Which option do you prefer?
>    - **Option A (Recommended): LaunchAgent** (Runs when user `nilbot` is logged in, no root/sudo needed, logs to `~/Library/Logs/nextdns-ip-updater.log`)
>    - **Option B: LaunchDaemon** (Runs on system boot under root, mimics systemd system service, requires `sudo` to register, logs to `/var/log/nextdns-ip-updater.log`)

---

## Proposed Changes

### Build
We will build the latest version of the binary for the host macOS architecture (`darwin/arm64`) using:
```bash
make build
```
Or specifically:
```bash
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "-w -s" -o nextdns-ip-updater .
```

### Option A: User LaunchAgent Setup (No root required)

#### [NEW] [com.nextdns.ip-updater.plist](file:///Users/nilbot/Library/LaunchAgents/com.nextdns.ip-updater.plist)
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.nextdns.ip-updater</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Users/nilbot/.local/bin/nextdns-ip-updater</string>
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
    <string>/Users/nilbot/Library/Logs/nextdns-ip-updater.log</string>
    <key>StandardErrorPath</key>
    <string>/Users/nilbot/Library/Logs/nextdns-ip-updater.log</string>
</dict>
</plist>
```

Commands to deploy:
```bash
# 1. Copy binary to local bin
cp nextdns-ip-updater ~/.local/bin/nextdns-ip-updater
chmod +x ~/.local/bin/nextdns-ip-updater

# 2. Register and start agent via launchctl
launchctl bootstrap gui/501 ~/Library/LaunchAgents/com.nextdns.ip-updater.plist
launchctl enable gui/501/com.nextdns.ip-updater
launchctl kickstart -k gui/501/com.nextdns.ip-updater
```

---

### Option B: System LaunchDaemon Setup (Requires root/sudo)

#### [NEW] [com.nextdns.ip-updater.plist](file:///Library/LaunchDaemons/com.nextdns.ip-updater.plist)
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
    <string>/var/log/nextdns-ip-updater.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/nextdns-ip-updater.log</string>
</dict>
</plist>
```

Commands to deploy:
```bash
# 1. Copy binary to usr local bin
sudo cp nextdns-ip-updater /usr/local/bin/nextdns-ip-updater
sudo chmod +x /usr/local/bin/nextdns-ip-updater

# 2. Register and start system daemon via launchctl
sudo launchctl bootstrap system /Library/LaunchDaemons/com.nextdns.ip-updater.plist
sudo launchctl enable system/com.nextdns.ip-updater
sudo launchctl kickstart -k system/com.nextdns.ip-updater
```

---

## Verification Plan

### Manual Verification
1. Verify the service is running using launchctl:
   - For Option A: `launchctl list | grep nextdns`
   - For Option B: `sudo launchctl list | grep nextdns`
2. Check the logs output for successful connection messages:
   - For Option A: `tail -n 20 ~/Library/Logs/nextdns-ip-updater.log`
   - For Option B: `tail -n 20 /var/log/nextdns-ip-updater.log`
3. Call the health metrics endpoint:
   - `curl http://localhost:48080/health`
   - `curl http://localhost:48080/ready`

## 🎯 Tasks & Progress Tracking

- `[x]` Build the latest macOS arm64 binary
- `[x]` Copy the binary to `~/.local/bin/nextdns-ip-updater` and make it executable
- `[x]` Create the plist configuration file at `~/Library/LaunchAgents/com.nextdns.ip-updater.plist`
- `[x]` Load and start the launchd agent
- `[x]` Verify execution by checking status and logs
- `[x]` Verify health check endpoints

## 🔍 Walkthrough & Verification

We configured the Go-based NextDNS IP Updater to run as an unattended background agent under macOS using `launchctl`.

## Changes Made

### 1. Build and Deployment
- Compiled the latest source code to a macOS arm64 binary:
  - Binary compiled to: `nextdns-ip-updater`
- Copied the compiled executable to the user-specific bin folder (avoiding root/sudo):
  - Location: `/Users/nilbot/.local/bin/nextdns-ip-updater`

### 2. launchd Configuration
- Created a custom user LaunchAgent property list file:
  - Path: [com.nextdns.ip-updater.plist](file:///Users/nilbot/Library/LaunchAgents/com.nextdns.ip-updater.plist)
  - Configuration details:
    - **Label**: `com.nextdns.ip-updater`
    - **Endpoint**: `https://link-ip.nextdns.io/YOUR_ID/YOUR_EXT_ID` (retrieved from `deploy/01-secrets.yaml`)
    - **Interval**: 300 seconds (5 minutes)
    - **Health Port**: 48080 (configured via `HEALTH_SERVER_PORT` to avoid conflict with port 8080)
    - **Logs path**: `/Users/nilbot/Library/Logs/nextdns-ip-updater.log`
    - **KeepAlive** & **RunAtLoad**: Enabled (starts automatically at login and restarts on crash)

### 3. Service Registration
- Registered the daemon with launchd:
  - Command: `launchctl bootstrap gui/501 ~/Library/LaunchAgents/com.nextdns.ip-updater.plist`
  - Command: `launchctl enable gui/501/com.nextdns.ip-updater`
  - Command: `launchctl kickstart -k gui/501/com.nextdns.ip-updater`

---

## Verification Results

### 1. launchctl Service Status
Checking service state:
```bash
launchctl print gui/501/com.nextdns.ip-updater
```
Status:
- **state** = `running`
- **pid** = `18607`
- **active count** = `1`

### 2. Service Logs
```json
{"build_time":"2026-06-10T15:44:52Z","endpoint":"https://link-ip.nextdns.io/YOUR_ID/YOUR_EXT_ID","health_port":48080,"interval_seconds":300,"level":"info","msg":"Starting NextDNS IP updater","time":"2026-06-10T16:51:08+01:00","version":"v0.1.6-1-g0736be9-dirty"}
{"level":"info","msg":"Health server starting","port":48080,"time":"2026-06-10T16:51:08+01:00"}
{"level":"info","msg":"Waiting for DNS to be ready...","time":"2026-06-10T16:51:09+01:00"}
{"level":"info","msg":"DNS is ready","time":"2026-06-10T16:51:09+01:00"}
{"endpoint":"https://link-ip.nextdns.io/YOUR_ID/YOUR_EXT_ID","level":"info","msg":"Successfully updated NextDNS","time":"2026-06-10T16:51:09+01:00"}
{"level":"info","msg":"Service marked as ready after first successful update","time":"2026-06-10T16:51:09+01:00"}
```

### 3. Health Endpoint Verification
Checking health checks:
```bash
curl -s http://localhost:48080/health
```
```json
{"healthy":true,"last_success":"2026-06-10T16:51:09.256836+01:00","uptime_seconds":2.378759209,"update_count":1,"error_count":0}
```

Checking readiness checks:
```bash
curl -s http://localhost:48080/ready
```
```json
{"ready":true,"message":"Service is ready"}
```

Checking metrics:
```bash
curl -s http://localhost:48080/metrics
```
```
# HELP nextdns_updates_total Total number of successful NextDNS updates
# TYPE nextdns_updates_total counter
nextdns_updates_total 1
```
