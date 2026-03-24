# XRP - Local Environment Proxy

XRP (Extensible Reverse Proxy) is a local development tool designed to automatically scan your system for running development servers and magically proxy them to `.local` domains.

## How it works

The core philosophy of XRP is "zero configuration". It natively leverages OS-level commands to detect running services.

### 1. Port Scanning
XRP contains an integrated OS-level scanner. Depending on your system, it uses different mechanisms:
- **Linux (`scanner/linux.go`)**: Reads from `/proc/net/tcp` to find listening ports and their inodes. It then scans `/proc/[pid]/fd` to correlate inodes with Process IDs (PIDs).
- **macOS (`scanner/darwin.go`)**: Leverages `lsof -iTCP -sTCP:LISTEN -Fpn` to fetch the listening ports and PIDs, and `ps -p [pid] -o comm=` to fetch the process name.
- **Windows (`scanner/windows.go`)**: Currently a dummy implementation for build compatibility.

### 2. Process Enrichment and Orchestration
The main entrypoint for scanning is `scanner.ScanProcesses()` in `scanner.go`. It orchestrates the OS-specific scanner and filters out noise:
- It ignores privileged ports (`< 1024`).
- It filters out system services (ports like 22, 80, 443, 3306, 5432) *unless* they are executed from a user (developer) directory (heuristic based on `$CWD`).
- It uses heuristics to determine the **Project Name** from the working directory (`$CWD`) by inspecting:
  - `package.json` -> `"name"`
  - `Cargo.toml` -> `[package] name`
  - `pyproject.toml` -> `[project] name`
  - **Fallback**: The base directory name of `$CWD`.
- It performs a lookup against a library of pre-defined ports (from `known-ports.json` loaded in `ports.go`) to identify the **Known App** (e.g. Next.js, React, Django).

### 3. Caddy Reverse Proxy
Instead of building a proxy engine from scratch, XRP acts as a control plane for **Caddy**. The proxy module (`proxy/caddy.go`) takes the enriched processes list and dynamically generates a JSON Caddy configuration.
It then sends a payload to Caddy's Admin API (`POST http://localhost:2019/load`) that looks roughly like this:
```json
{
  "apps": {
    "http": {
      "servers": {
        "xrp_server": {
          "listen": [":80"],
          "routes": [
            {
              "match": [{"host": ["myproject.local"]}],
              "handle": [{"handler": "reverse_proxy", "upstreams": [{"dial": "localhost:3000"}]}]
            }
          ]
        }
      }
    }
  }
}
```

### 4. Interactive CLI Dashboard
The user interfaces XRP using the `xrp` CLI tool (`cmd/xrp/main.go`). It:
1. Orchestrates the process scanning.
2. Interacts with the Caddy proxy configuration API.
3. Formats and prints out a beautifully colored dashboard listing the discovered ports, PIDs, Process Names, Projects, and known Apps.
4. Provides clickable `http://[project].local` URLs directly in the terminal to jump into your development endpoints effortlessly!

## Usage
Simply run the binary!
```bash
go run ./cmd/xrp
```
