# Halo Proxy — Zero-Config Local Reverse Proxy

> **Halo Proxy** (command: `halo`) automatically discovers your running local development servers and routes them to clean `.localhost` domains with HTTPS — no config files, no manual hosts editing, no port memorizing.

---

## ✨ Features

- 🔍 **Auto-discovery** — scans running ports and maps them to named projects
- 🌐 **`.localhost` HTTPS** — uses Caddy + mkcert for trusted local TLS out of the box
- 🏷️ **Per-project TLD overrides** — give any service its own custom domain (e.g. `jellyfin.media`)
- 🚀 **Interactive TUI** — live dashboard with keyboard navigation
- 🌍 **Cloudflared tunnels** — share any local service publicly in one keypress
- 📦 **Self-bootstrapping** — downloads Caddy, mkcert, cloudflared automatically if missing
- 💻 **Cross-platform** — Windows, Linux, macOS
- ⚡ **Zero restart** — TLD changes take effect within one poll cycle (5s)

> **Why `.localhost`?** Anything under `.localhost` is resolved to loopback by the OS and every modern browser (RFC 6761) — no `/etc/hosts` editing and no root needed. Custom TLDs (`*.media`, `*.dev`, …) are supported too and are written to the hosts file, which needs root/Administrator (or a writable `HALO_HOSTS_PATH`).

---

## 🚀 Quick Start

```sh
# Install halo to PATH (no admin rights needed on Windows)
halo install

# Start the background daemon
halo start

# Open the interactive TUI dashboard
halo tui

# Or list services in the terminal
halo list
```

---

## 📖 Commands

| Command | Description |
|---|---|
| `halo start` | Start the Halo Proxy daemon in the background |
| `halo stop` | Stop the running daemon |
| `halo reload` | Restart the daemon (picks up config changes) |
| `halo status` | Check if daemon is running |
| `halo list` | Print all detected services and their URLs |
| `halo open [project]` | Open a project URL in the browser |
| `halo tui` | Launch the interactive TUI dashboard |
| `halo share [project]` | Share a service publicly via cloudflared tunnel |
| `halo unshare [project]` | Stop a cloudflared tunnel |
| `halo set-tld [project] [tld]` | Set a custom TLD for a specific project |
| `halo install` | Install the `halo` binary to your user PATH |
| `halo version` | Print version |
| `halo help` | Show help |

> On the **first** `halo start`, halo runs `mkcert -install` to trust a local CA. On Linux/macOS this is the only step that needs `sudo` (a one-time password prompt).

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

Halo Proxy supports per-project TLD overrides. The default TLD is `.localhost` (zero-config, no hosts edit needed). Custom TLDs are added to the hosts file, which needs root/Administrator — if the hosts file isn't writable, halo logs a warning and only `.localhost` names resolve.

**Via CLI:**
```sh
halo set-tld jellyfin media
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

Config is persisted to `~/.config/halo/config.toml`:
```toml
tld = ".localhost"

[project_tlds]
  jellyfin = "media"
  myapp = "dev"
```

---

## ⚙️ Configuration

Config file: `~/.config/halo/config.toml`

| Key | Default | Description |
|---|---|---|
| `tld` | `.localhost` | Default TLD for all discovered services |
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

halo uses OS-native mechanisms to find listening ports:

| OS | Method |
|---|---|
| **Linux** | Reads `/proc/net/tcp` + `/proc/net/tcp6` (filters `0A` = LISTEN state), correlates inodes via `/proc/[pid]/fd` |
| **macOS** | `lsof -iTCP -sTCP:LISTEN` |
| **Windows** | `netstat -ano` + `tasklist` + `wmic` path correlation |

### Project Name Resolution

For each discovered port, halo resolves a project name by inspecting the process CWD:
1. `package.json` → `name` field
2. `Cargo.toml` → `[package] name`
3. `pyproject.toml` → `[project]` or `[tool.poetry]` name
4. Fallback: base directory name

### Noise Filtering

halo automatically excludes:
- System processes (`svchost`, `lsass`, `csrss`, etc.)
- Ephemeral ports (49152–65535) for unknown processes
- Common noise apps (Spotify, OneDrive, etc.)
- halo itself (no recursive proxy entries)

### Caddy Integration

halo acts as a **control plane for Caddy**. On each poll it:
1. Builds a Caddy JSON config with one reverse proxy route per service
2. Runs two logical servers: plain HTTP on `http_port` (308-redirects to HTTPS) and a TLS server on `https_port` that terminates with the local mkcert certs
3. Posts the config to the Caddy admin API (hot-reload, zero downtime)
4. Updates `/etc/hosts` (or `C:\Windows\System32\drivers\etc\hosts`) with new entries

The reverse-proxy upstream uses the exact loopback address the service was discovered on (`127.0.0.1` or `::1`), so IPv4-only and IPv6-only dev servers both work.

### Dependency Management

On first start, halo automatically downloads and caches required binaries to `~/.cache/halo/bin/`:

| Binary | Version | Purpose |
|---|---|---|
| `caddy` | 2.9.1 | Reverse proxy engine |
| `mkcert` | 1.4.4 | Local CA + certificate generation |
| `cloudflared` | 2024.12.0 | Public tunnel sharing |

Downloads are concurrent, context-aware (5-minute timeout), and checksum-verified when a release provides hashes.

---

## 🌐 Cloudflared Sharing

Share any local service publicly with a single command:

```sh
halo share jellyfin
# → https://random-name.trycloudflare.com
```

Or use the TUI — press `s` on any row.

Active tunnels are shown in the TUI `STATUS / TUNNEL` column.

Stop a tunnel:
```sh
halo unshare jellyfin
```

---

## 🛠️ IPC Architecture

The daemon exposes a TCP IPC server on `127.0.0.1:40192` (JSON-RPC protocol). This powers:
- CLI commands (`halo list`, `halo share`, etc.)
- TUI live updates
- VS Code extension
- _(upcoming)_ Tauri desktop GUI

---

## 📦 VS Code Extension

The bundled VS Code extension (`vscode-extension/`) shows active services in the sidebar and lets you open them in the browser. It connects to the daemon via the TCP IPC server.

---

## 🔒 Security

- All local HTTPS uses **mkcert** certificates signed by a local CA installed in your system trust store
- Downloaded binaries are checked against pinned SHA256 checksums when populated (the checksum tables in `internal/deps` ship blank, so verification is skipped until a release fills them in)
- No telemetry, no external calls except dependency downloads and cloudflared tunnels
- Caddy admin API (`localhost:2019`) is bound to loopback only

---

## 🏗️ Architecture

```
halo (cli)
├── cmd/halo/cli/        — Cobra commands (start, stop, list, tui, share, set-tld, install...)
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

GPL-3.0-or-later (see [LICENSE](LICENSE)).
