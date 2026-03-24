# XRP Dashboard for VS Code

This is the companion extension for **XRP**, the local zero-config reverse proxy daemon. It allows you to seamlessly view and access all your magically mapped `.local` domains right from your editor's sidebar!

## Features

- **Real-Time Discovery**: Communicates locally with the XRP Daemon via Unix Sockets to instantly show you what development servers are active on your machine.
- **Sidebar Integration**: Introduces a clean "XRP" tab in your Activity Bar.
- **Status Bar Indicator**: Always know exactly how many services are currently reverse proxied by XRP in the bottom right corner of VS Code.
- **One-Click Open**: Launch your mapped `https://project.local` URLs directly from the tree view inline actions.
- **Lightweight**: Uses high-performance local IPC to fetch metadata, drawing zero overhead.

## Requirements

You must have the core **XRP CLI Daemon** installed and running on your system for this extension to populate.

Run `xrp start` in your terminal to start the daemon in the background before using this extension.

## Usage

1. Start your local development server (Next.js, Vue, Django, Rails etc).
2. Start the XRP Background Daemon (`xrp start`).
3. Click the XRP Icon in VS Code Activity Bar.
4. Click the "Open in Browser" button directly next to your active projects!
