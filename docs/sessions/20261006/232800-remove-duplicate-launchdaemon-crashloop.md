# Remove duplicate LaunchDaemon crash-looping since install

- **Date**: 2026-10-06 23:28
- **Host**: `nilbot` Mac (macOS, user `nilbot`, uid 501)

## What was reported

Mole (disk cleaner / startup-app inspector) listed the NextDNS updater daemon as
being in an error state, but its UI did not show the error text.

## What was actually on the machine

Two independent services ran the same updater:

| Label | Type | Plist | Binary | State at investigation |
|---|---|---|---|---|
| `com.nextdns.ip-updater` | LaunchAgent (gui/501) | `~/Library/LaunchAgents/com.nextdns.ip-updater.plist` | `~/.local/bin/nextdns-ip-updater` | Working, PID 1252, up since 2026-09-28 boot |
| `net.nilbot.nextdns-ip-updater` | LaunchDaemon (system) | `/Library/LaunchDaemons/net.nilbot.nextdns-ip-updater.plist` | `/opt/nextdns-ip-updater/nextdns-ip-updater` | Crash-looping, exit code 1 |

Mole reports only the launchd status integer, which is why it showed "error" with
no reason. `launchctl list` printed `- 1 net.nilbot.nextdns-ip-updater`: no PID,
last exit status 1.

## Root cause

`sudo launchctl print system/net.nilbot.nextdns-ip-updater` showed
`state = spawn scheduled`, `runs = 64314`, `last exit code = 1`, and
`/var/log/nextdns-ip-updater.log` had 165,774 lines — all of them identical,
written every ~10 seconds (launchd's minimum-runtime throttle), the earliest
surviving line being `2026-09-17T01:35:45+01:00`:

```json
{"level":"error","msg":"NEXTDNS_ENDPOINT environment variable is not set","time":"2026-10-06T23:24:27+01:00"}
```

`grep -c "Successfully updated" /var/log/nextdns-ip-updater.log` returned `0`:
this service never completed one update.

Two independent defects, either one of which is sufficient to break it:

1. **Empty endpoint.** `/etc/nextdns-ip-updater.conf:7` is the unmodified shipped
   template — `NEXTDNS_ENDPOINT=` with no value. `main.go:157-161` reads
   `os.Getenv("NEXTDNS_ENDPOINT")` and calls `os.Exit(1)` when it is empty, which
   is the exit code launchd recorded.

2. **The plist wrapper never exported the config.** `net.nilbot.nextdns-ip-updater.plist:11`
   runs `/bin/bash -c 'source /etc/nextdns-ip-updater.conf; ... exec /opt/nextdns-ip-updater/nextdns-ip-updater'`.
   The config file assigns plain shell variables with no `export`, so they never
   enter the environment of the `exec`'d binary. Filling in the endpoint alone
   would therefore not have fixed it. Demonstrated:

   ```
   $ printf 'FOO=bar\n' > /tmp/t
   $ /bin/bash -c 'source /tmp/t; echo "shell=[${FOO}]"; env | grep -c "^FOO="'
   shell=[bar]
   0
   ```

   `export -p` after sourcing the real config also showed no exported
   `NEXTDNS_ENDPOINT`.

The LaunchDaemon was superseded the same day it was installed. File times:
`/Library/LaunchDaemons/net.nilbot.nextdns-ip-updater.plist` and
`/opt/nextdns-ip-updater/nextdns-ip-updater` are both `Jun 10 16:36`, while
`~/.local/bin/nextdns-ip-updater` is `Jun 10 16:45` and the LaunchAgent plist is
`Jun 10 16:50`; the session record
`docs/sessions/20260610/165117-setup-launchd-daemon.md:27` shows Option A
(LaunchAgent) was the choice. The daemon was left installed and loaded, and has
been respawning ever since.

Nothing in this repository or in the dotfiles checkout provisions that daemon —
`grep -rn "nextdns" /Users/nilbot/dotfiles` returns nothing, and no file here
references the `net.nilbot.nextdns-ip-updater` label — so it was a one-off
install that provisioning will not recreate.

## Action taken

Chose to keep the working LaunchAgent and remove the broken duplicate.

```bash
sudo launchctl bootout system/net.nilbot.nextdns-ip-updater
# archived, then removed:
#   /Library/LaunchDaemons/net.nilbot.nextdns-ip-updater.plist
#   /var/log/nextdns-ip-updater.log        (18,236,020 bytes -> 496,850 gzipped)
```

Backup location:
`~/.local/backups/nextdns-ip-updater-daemon-2026-10-06/`

## Verification after removal

- `launchctl list` (user domain): only `com.nextdns.ip-updater` (status 0) and
  `dev.nilbot.filebrowser`.
- `sudo launchctl list` (system domain): no nextdns job — `print` returns
  `Could not find service`.
- The log file size was byte-identical across a 12-second window before removal,
  confirming the respawn loop had stopped.
- `curl http://127.0.0.1:48080/health` →
  `{"healthy":true,"ready":true,"update_count":2151,"last_success":"2026-10-06T23:25:19","error_count":10}`
  and `/ready` → `{"ready":true}`. `update_count` advanced 2150 → 2151 across the
  change, so the surviving agent kept updating.
- The `error_count: 10` are transient
  `net/http: timeout awaiting response headers` blips against the NextDNS
  endpoint, not a fault; the health endpoint tolerates them.

## Leftovers (inert, not removed)

- `/opt/nextdns-ip-updater/nextdns-ip-updater` — 6,119,346 bytes, root-owned.
- `/etc/nextdns-ip-updater.conf` — the empty template, plus the
  `/etc/default/nextdns-ip-updater` symlink to it.

Neither is referenced by anything anymore. If a system-wide (boot-time, pre-login)
service is wanted later, the correct fix is to set the endpoint **and** make it
reach the process — either `export` in the config, or drop the bash wrapper and
put `EnvironmentVariables` in the plist, as the LaunchAgent does.

## Restoring, if ever needed

```bash
sudo cp ~/.local/backups/nextdns-ip-updater-daemon-2026-10-06/net.nilbot.nextdns-ip-updater.plist /Library/LaunchDaemons/
# fix BOTH defects first, or it will simply crash-loop again:
#   1. set NEXTDNS_ENDPOINT in /etc/nextdns-ip-updater.conf
#   2. add `export` to those assignments (or replace the wrapper with EnvironmentVariables)
sudo launchctl bootstrap system /Library/LaunchDaemons/net.nilbot.nextdns-ip-updater.plist
```
