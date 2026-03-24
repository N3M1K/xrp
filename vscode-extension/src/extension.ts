import * as vscode from 'vscode';
import { XrpProvider, XrpTreeItem } from './XrpProvider';
import { fetchProcesses, sendCommand } from './socket';

let statusBarItem: vscode.StatusBarItem;
let refreshInterval: NodeJS.Timeout;

export function activate(context: vscode.ExtensionContext) {
  const xrpProvider = new XrpProvider();
  
  vscode.window.registerTreeDataProvider('xrp-services', xrpProvider);

  context.subscriptions.push(
    vscode.commands.registerCommand('xrp.refresh', () => {
      xrpProvider.refresh();
      updateStatusBar();
    }),
    vscode.commands.registerCommand('xrp.open', async (item: XrpTreeItem) => {
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

export function deactivate() {
  if (refreshInterval) {
    clearInterval(refreshInterval);
  }
}
