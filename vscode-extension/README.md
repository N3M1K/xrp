# Halo Proxy Dashboard for VS Code

This is the companion extension for **Halo Proxy**, the local zero-config reverse proxy daemon. It allows you to seamlessly view and access all your magically mapped `.localhost` (and custom-TLD) domains right from your editor's sidebar!

## Features

- **Real-Time Discovery**: Communicates locally with the Halo Proxy Daemon over a loopback TCP socket (127.0.0.1:40192) to instantly show you what development servers are active on your machine.
- **Sidebar Integration**: Introduces a clean "Halo Proxy" tab in your Activity Bar.
- **Status Bar Indicator**: Always know exactly how many services are currently reverse proxied by Halo Proxy in the bottom right corner of VS Code.
- **One-Click Open**: Launch the exact URL reported by the daemon (respecting custom TLDs) directly from the tree view inline actions.
- **Lightweight**: Uses high-performance local IPC to fetch metadata, drawing zero overhead.

## Requirements

You must have the core **Halo Proxy CLI Daemon** installed and running on your system for this extension to populate.

Run `halo start` in your terminal to start the daemon in the background before using this extension.

## Usage

1. Start your local development server (Next.js, Vue, Django, Rails etc).
2. Start the Halo Proxy Background Daemon (`halo start`).
3. Click the Halo Proxy Icon in VS Code Activity Bar.
4. Click the "Open in Browser" button directly next to your active projects!


## License

This extension is licensed under the [GNU General Public License v3.0](LICENSE).