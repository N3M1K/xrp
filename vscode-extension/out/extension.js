// src/extension.ts
import * as vscode2 from "vscode";

// src/XrpProvider.ts
import * as vscode from "vscode";

// src/socket.ts
import * as net from "net";
import * as os from "os";
import * as path from "path";
function getSocketPath() {
  return path.join(os.tmpdir(), "xrp.sock");
}
async function sendCommand(cmd, args) {
  return new Promise((resolve, reject) => {
    const client = net.createConnection({ path: getSocketPath() });
    client.on("connect", () => {
      client.write(JSON.stringify({ cmd, args }) + `
`);
    });
    let buffer = "";
    client.on("data", (data) => {
      buffer += data.toString();
    });
    client.on("end", () => {
      try {
        const resp = JSON.parse(buffer);
        resolve(resp);
      } catch (err) {
        reject(err);
      }
    });
    client.on("error", (err) => {
      reject(err);
    });
  });
}
async function fetchProcesses() {
  try {
    const resp = await sendCommand("list");
    if (resp.success) {
      if (typeof resp.data === "string") {
        return JSON.parse(resp.data);
      }
      return resp.data;
    }
  } catch (err) {}
  return [];
}

// src/XrpProvider.ts
class XrpProvider {
  _onDidChangeTreeData = new vscode.EventEmitter;
  onDidChangeTreeData = this._onDidChangeTreeData.event;
  refresh() {
    this._onDidChangeTreeData.fire();
  }
  getTreeItem(element) {
    return element;
  }
  async getChildren(element) {
    if (element) {
      return [];
    }
    const processes = await fetchProcesses();
    if (processes.length === 0) {
      return [new XrpTreeItem("No running services detected.", "", "", vscode.TreeItemCollapsibleState.None)];
    }
    return processes.map((p) => {
      const url = p.ProjectName ? `https://${p.ProjectName}.local` : `http://localhost:${p.Port}`;
      const label = p.ProjectName ? `${p.ProjectName} (${p.KnownApp || "Unknown"})` : `${p.ProcessName}:${p.Port}`;
      const item = new XrpTreeItem(label, url, `Port: ${p.Port} | PID: ${p.PID}`, vscode.TreeItemCollapsibleState.None);
      if (p.ProjectName || p.Port) {
        item.contextValue = "xrp-service";
      }
      return item;
    });
  }
}

class XrpTreeItem extends vscode.TreeItem {
  label;
  url;
  tooltip;
  collapsibleState;
  constructor(label, url, tooltip, collapsibleState) {
    super(label, collapsibleState);
    this.label = label;
    this.url = url;
    this.tooltip = tooltip;
    this.collapsibleState = collapsibleState;
    if (url) {
      this.description = url;
    }
  }
}

// src/extension.ts
var statusBarItem;
var refreshInterval;
function activate(context) {
  const xrpProvider = new XrpProvider;
  vscode2.window.registerTreeDataProvider("xrp-services", xrpProvider);
  context.subscriptions.push(vscode2.commands.registerCommand("xrp.refresh", () => {
    xrpProvider.refresh();
    updateStatusBar();
  }), vscode2.commands.registerCommand("xrp.open", async (item) => {
    if (item && item.url) {
      try {
        await sendCommand("open", { url: item.url });
      } catch (e) {
        vscode2.env.openExternal(vscode2.Uri.parse(item.url));
      }
    }
  }));
  statusBarItem = vscode2.window.createStatusBarItem(vscode2.StatusBarAlignment.Right, 100);
  context.subscriptions.push(statusBarItem);
  updateStatusBar();
  refreshInterval = setInterval(() => {
    xrpProvider.refresh();
    updateStatusBar();
  }, 5000);
}
async function updateStatusBar() {
  const processes = await fetchProcesses();
  if (processes.length > 0) {
    statusBarItem.text = `$(globe) XRP: ${processes.length}`;
    statusBarItem.tooltip = "Local proxy domains are active";
    statusBarItem.show();
  } else {
    statusBarItem.hide();
  }
}
function deactivate() {
  if (refreshInterval) {
    clearInterval(refreshInterval);
  }
}
export {
  deactivate,
  activate
};
