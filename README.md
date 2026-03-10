# GoBeacon

> A modular Command & Control (C2) framework built in Go for offensive security research and red team operations.

![Go](https://img.shields.io/badge/Go-1.21-blue) ![Platform](https://img.shields.io/badge/platform-Windows-lightgrey) ![Purpose](https://img.shields.io/badge/purpose-research-red)

---

## ⚠️ Disclaimer

This project was developed strictly for **educational and research purposes**. It is intended to help understand offensive techniques in order to build better defenses. Do not use on any system without **explicit written authorization**. The author is not responsible for misuse.

---

## Overview

GoBeacon is a staged C2 framework that uses the Telegram Bot API as its communication channel. The operator never communicates directly with the implant — all traffic flows through Telegram's infrastructure, making the beacon appear as legitimate outbound traffic to `api.telegram.org`.

The project was designed to study:
- C2 communication patterns and detection evasion
- Keylogging techniques on Windows at the API level
- Staged payload delivery to minimize initial footprint
- OpSec considerations for red team operations

---

## Architecture

```
Operator (Whonix Workstation)
        │
        │  Tor Network
        ▼
  Telegram API
  (api.telegram.org)
        │
        │  Outbound HTTPS (looks legitimate)
        ▼
  Beacon (target machine)
```

**Why Telegram as a C2 channel?**

Most corporate environments and EDR solutions do not flag or block outbound traffic to `api.telegram.org`. This eliminates the need for attacker-controlled infrastructure (no C2 server to host, no domain to register, no IP to burn) and significantly reduces the operational footprint.

**Why Whonix as the operator workstation?**

Whonix routes all traffic through Tor via its Gateway VM, ensuring the operator's real IP is never exposed — not even to Telegram. The Workstation VM has no direct network access, so even if the workstation is compromised, the operator's identity remains protected.

---

## Components

The framework is divided into stages to reduce detection surface:

| File | Language | Description |
|------|----------|-------------|
| `beacon.go` | Go | Core C2 implant. Connects to Telegram Bot API, handles shell sessions, executes commands |
| `keylogger.go` | Go | Standalone keylogger. Uses `GetAsyncKeyState` from `user32.dll` directly. Logs are XOR-obfuscated and Base64-encoded |
| `persist.bat` | Batch | Installs persistence via Windows Registry (`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`) |
| `recon.bat` | Batch | Obfuscated WinPEAS wrapper for local privilege escalation enumeration |

### Staged Deployment

```
Stage 1: beacon.exe    →  Establish C2 channel
Stage 2: persist.bat   →  Install persistence
Stage 3: keylogger.exe →  Deploy keylogger (via beacon shell)
Stage 4: recon.bat     →  Run enumeration (if escalation needed)
```

Separating components into stages means that if one artifact is detected, the others are not immediately associated with it.

---

## Beacon Features

- **Multi-session support** — Control multiple machines from a single Telegram bot using session ID prefixes
- **Session-aware shell** — Maintains working directory state across commands
- **File exfiltration** — `/download` command sends files directly to the operator via Telegram
- **Broadcast commands** — `/all` sends a command to all active beacons simultaneously
- **Hidden execution** — Uses `CREATE_NO_WINDOW` flag to suppress console window on Windows
- **Command blocklist** — Prevents destructive commands (`rm -rf`, `format`, `shutdown`, etc.)

## Keylogger Features

- Direct Windows API calls via `syscall` (no external dependencies)
- Random filename (`cache_XXXXXX.bin`) to avoid static detection
- File hidden using `SetFileAttributesW` (Hidden attribute)
- XOR obfuscation + Base64 encoding on logged data
- Timestamped entries for reconstruction
- Silent failure — exits without output if it cannot initialize

---

## Technical Notes

### Why Go?

Go compiles to a single static binary with no runtime dependencies. This simplifies deployment and reduces detection surface compared to interpreted languages like Python. Cross-compilation also allows building Windows targets from a Linux/Whonix environment.

### Keylogger Detection Considerations

The current XOR key (`0x55`) is static and trivially reversible by a malware analyst. A production-grade implementation would use a derived key or AES encryption. This was intentionally kept simple for readability and learning purposes.

### OpSec Tradeoffs

Using Telegram introduces a dependency on a third-party service. If Telegram blocks the bot token or the account is reported, the C2 channel is lost. A more resilient implementation would include a fallback channel or domain fronting.

---

## Setup

### Requirements
- Go 1.21+
- A Telegram Bot Token (from [@BotFather](https://t.me/botfather))
- Your Telegram User ID

### Configuration

> **TODO:** Credentials are currently hardcoded in `loadConfig()`. Move to environment variables or an external config file before any production use.

For now, edit directly in `loadConfig()`:


```json
{
  "bot_token": "your_token_here",
  "admin_ids": [123456789]
}
```

### Build

```bash
# Build for Windows from Linux
GOOS=windows GOARCH=amd64 go build -o beacon.exe beacon.go
GOOS=windows GOARCH=amd64 go build -o keylogger.exe keylogger.go
```

---

## Author

**Alejandro** — Mechatronics Engineering student with a focus on offensive security research.

- 🔗 [LinkedIn](#)
- 🐙 [GitHub](#)
- 🎯 [TryHackMe](#)

---

## License

For educational use only. See [LICENSE](LICENSE).
