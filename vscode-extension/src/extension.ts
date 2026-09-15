import * as vscode from 'vscode';
import { HaloProvider, HaloTreeItem } from './HaloProvider';
import { fetchProcesses, sendCommand } from './socket';

let statusBarItem: vscode.StatusBarItem;
let refreshInterval: NodeJS.Timeout;

export function activate(context: vscode.ExtensionContext) {
  const haloProvider = new HaloProvider();
  
  vscode.window.registerTreeDataProvider('halo-services', haloProvider);

  context.subscriptions.push(
    vscode.commands.registerCommand('halo.refresh', () => {
      haloProvider.refresh();
      updateStatusBar();
    }),
    vscode.commands.registerCommand('halo.open', async (item: HaloTreeItem) => {
      if (item && item.url) {
        try {
          // Tell daemon to open it via RPC
          await sendCommand('open', { url: item.url });
        } catch(e) {
          // Fallback, open natively if daemon fails mapping it
          vscode.env.openExternal(vscode.Uri.parse(item.url));
        }
      }
    })
  );

  statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 100);
  context.subscriptions.push(statusBarItem);
  
  // Initial fill
  updateStatusBar();
  
  refreshInterval = setInterval(() => {
    haloProvider.refresh();
    updateStatusBar();
  }, 5000);
}

async function updateStatusBar() {
  const processes = await fetchProcesses();
  if (processes.length > 0) {
    statusBarItem.text = `$(globe) Halo: ${processes.length}`;
    statusBarItem.tooltip = "Local proxy domains are active";
    statusBarItem.show();
  } else {
    statusBarItem.hide();
  }
}

export function deactivate() {
  if (refreshInterval) {
    clearInterval(refreshInterval);
  }
}
