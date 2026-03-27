# XRP — Zero-Config Local Reverse Proxy

> **xrp** (xxdev's Reverse Proxy) automatically discovers your running local development servers and routes them to clean `.local` domains with HTTPS — no config files, no manual hosts editing, no port memorizing.

---

## ✨ Features

- 🔍 **Auto-discovery** — scans running ports and maps them to named projects
- 🌐 **`.local` HTTPS** — uses Caddy + mkcert for trusted local TLS out of the box
- 🏷️ **Per-project TLD overrides** — give any service its own custom domain (e.g. `jellyfin.media`)
- 🚀 **Interactive TUI** — live dashboard with keyboard navigation
- 🌍 **Cloudflared tunnels** — share any local service publicly in one keypress
- 📦 **Self-bootstrapping** — downloads Caddy, mkcert, cloudflared automatically if missing
- 💻 **Cross-platform** — Windows, Linux, macOS
- ⚡ **Zero restart** — TLD changes take effect within one poll cycle (5s)

---

## 🚀 Quick Start

```sh
# Install xrp to PATH (no admin rights needed on Windows)
xrp install

# Start the background daemon
xrp start

# Open the interactive TUI dashboard
xrp tui

# Or list services in the terminal
xrp list
```

---

## 📖 Commands

| Command | Description |
|---|---|
| `xrp start` | Start the XRP daemon in the background |
| `xrp stop` | Stop the running daemon |
| `xrp status` | Check if daemon is running |
| `xrp list` | Print all detected services and their URLs |
| `xrp tui` | Launch the interactive TUI dashboard |
| `xrp share [project]` | Share a service publicly via cloudflared tunnel |
| `xrp unshare [project]` | Stop a cloudflared tunnel |
| `xrp set-tld [project] [tld]` | Set a custom TLD for a specific project |
| `xrp install` | Install the `xrp` binary to your user PATH |
| `xrp version` | Print version |
| `xrp help` | Show help |

---

## 🖥️ TUI Keyboard Shortcuts

| Key | Action |
|---|---|
| `↑` / `↓` or `k` / `j` | Navigate rows |
| `Enter` / `o` | Open service URL in browser |
| `t` | Set custom TLD for selected project |
| `s` | Start cloudflared tunnel for selected project |
| `u` | Stop cloudflared tunnel |
| `c` | Copy tunnel URL to clipboard |
| `q` / `Ctrl+C` | Quit |

---

## 🏷️ Custom TLDs

XRP supports per-project TLD overrides. The default TLD is `.local`.

**Via CLI:**
```sh
xrp set-tld jellyfin media
# → https://jellyfin.media
```

**Via TUI:**
1. Select a row with `↑`/`↓`
2. Press `t`
3. Type the new TLD (e.g. `media`)
4. Press `Enter` to confirm

Changes take effect within the next poll cycle (~5 seconds) — the daemon automatically:
- Generates a new wildcard mkcert certificate for the new TLD
- Updates the hosts file with the new hostname
- Reloads the Caddy configuration

Config is persisted to `~/.config/xrp/config.toml`:
```toml
tld = ".local"

[project_tlds]
  jellyfin = "media"
  myapp = "dev"
```

---

## ⚙️ Configuration

Config file: `~/.config/xrp/config.toml`

| Key | Default | Description |
|---|---|---|
| `tld` | `.local` | Default TLD for all discovered services |
| `poll_interval` | `5` | Seconds between port scans |
| `caddy_port` | `2019` | Caddy admin API port |
| `http_port` | `80` | HTTP listening port (requires admin/root) |
| `https_port` | `443` | HTTPS listening port (requires admin/root) |
| `log_level` | `info` | Log verbosity |

> **Note:** Binding ports 80 and 443 requires elevated privileges.
> - **Windows**: Run terminal as Administrator
> - **Linux**: Run `sudo setcap cap_net_bind_service=+ep $(which caddy)` once

---

## 🔍 How It Works

### Port Discovery

xrp uses OS-native mechanisms to find listening ports:

| OS | Method |
|---|---|
| **Linux** | Reads `/proc/net/tcp` (filters `0A` = LISTEN state), correlates inodes via `/proc/[pid]/fd` |
| **macOS** | `lsof -iTCP -sTCP:LISTEN` |
| **Windows** | `netstat -ano` + `tasklist` + `wmic` path correlation |

### Project Name Resolution

For each discovered port, xrp resolves a project name by inspecting the process CWD:
1. `package.json` → `name` field
2. `Cargo.toml` → `[package] name`
3. `pyproject.toml` → `[project]` or `[tool.poetry]` name
4. Fallback: base directory name

### Noise Filtering

xrp automatically excludes:
- System processes (`svchost`, `lsass`, `csrss`, etc.)
- Ephemeral ports (49152–65535) for unknown processes
- Common noise apps (Spotify, OneDrive, etc.)
- xrp itself (no recursive proxy entries)

### Caddy Integration

xrp acts as a **control plane for Caddy**. On each poll it:
1. Builds a Caddy JSON config with one reverse proxy route per service
2. Posts it to `http://localhost:2019/load` (hot-reload, zero downtime)
3. Updates `/etc/hosts` (or `C:\Windows\System32\drivers\etc\hosts`) with new entries

### Dependency Management

On first start, xrp automatically downloads and caches required binaries to `~/.cache/xrp/bin/`:

| Binary | Version | Purpose |
|---|---|---|
| `caddy` | 2.9.1 | Reverse proxy engine |
| `mkcert` | 1.4.4 | Local CA + certificate generation |
| `cloudflared` | 2024.12.0 | Public tunnel sharing |

Downloads are concurrent, context-aware (5-minute timeout), and validated for integrity.

---

## 🌐 Cloudflared Sharing

Share any local service publicly with a single command:

```sh
xrp share jellyfin
# → https://random-name.trycloudflare.com
```

Or use the TUI — press `s` on any row.

Active tunnels are shown in the TUI `STATUS / TUNNEL` column.

Stop a tunnel:
```sh
xrp unshare jellyfin
```

---

## 🛠️ IPC Architecture

The daemon exposes a TCP IPC server on `127.0.0.1:40192` (JSON-RPC protocol). This powers:
- CLI commands (`xrp list`, `xrp share`, etc.)
- TUI live updates
- VS Code extension
- _(upcoming)_ Tauri desktop GUI

---

## 📦 VS Code Extension

The bundled VS Code extension (`vscode-extension/`) shows active services in the sidebar and lets you open them in the browser. It connects to the daemon via the TCP IPC server.

---

## 🔒 Security

- All local HTTPS uses **mkcert** certificates signed by a local CA installed in your system trust store
- Downloaded binaries are verified with SHA256 checksums (populated per release)
- No telemetry, no external calls except dependency downloads and cloudflared tunnels
- Caddy admin API (`localhost:2019`) is bound to loopback only

---

## 🏗️ Architecture

```
xrp (cli)
├── cmd/xrp/cli/        — Cobra commands (start, stop, list, tui, share, set-tld, install...)
├── internal/
│   ├── daemon/         — Background process orchestrator
│   ├── scanner/        — OS-specific port & process discovery
│   ├── proxy/          — Caddy config generation & API client
│   ├── ssl/            — mkcert wrapper, cert generation
│   ├── hosts/          — /etc/hosts manager
│   ├── tunnel/         — cloudflared lifecycle
│   ├── socket/         — TCP IPC server (127.0.0.1:40192)
│   ├── deps/           — Binary dependency manager
│   ├── config/         — Config loading & mutations
│   └── tui/            — Bubble Tea TUI model
└── vscode-extension/   — VS Code sidebar extension
```

---

## 📄 License

MIT
