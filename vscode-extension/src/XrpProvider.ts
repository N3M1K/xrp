import * as vscode from 'vscode';
import { fetchProcesses, Process } from './socket';

export class XrpProvider implements vscode.TreeDataProvider<XrpTreeItem> {
  private _onDidChangeTreeData: vscode.EventEmitter<XrpTreeItem | undefined | void> = new vscode.EventEmitter<XrpTreeItem | undefined | void>();
  readonly onDidChangeTreeData: vscode.Event<XrpTreeItem | undefined | void> = this._onDidChangeTreeData.event;

  refresh(): void {
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(element: XrpTreeItem): vscode.TreeItem {
    return element;
  }

  async getChildren(element?: XrpTreeItem): Promise<XrpTreeItem[]> {
    if (element) {
      return [];
    }

    const processes = await fetchProcesses();
    if (processes.length === 0) {
      return [new XrpTreeItem("No running services detected.", "", "", vscode.TreeItemCollapsibleState.None)];
    }

    return processes.map(p => {
      const url = p.ProjectName ? `https://${p.ProjectName}.local` : `http://localhost:${p.Port}`;
      const label = p.ProjectName ? `${p.ProjectName} (${p.KnownApp || 'Unknown'})` : `${p.ProcessName}:${p.Port}`;
      
      const item = new XrpTreeItem(
        label,
        url,
        `Port: ${p.Port} | PID: ${p.PID}`,
        vscode.TreeItemCollapsibleState.None
      );
      
      // We set contextValue to xrp-service so our package.json knows when to show the inline open icon
      if (p.ProjectName || p.Port) {
        item.contextValue = "xrp-service";
      }
      return item;
    });
  }
}

export class XrpTreeItem extends vscode.TreeItem {
  constructor(
    public readonly label: string,
    public readonly url: string,
    public readonly tooltip: string,
    public readonly collapsibleState: vscode.TreeItemCollapsibleState
  ) {
    super(label, collapsibleState);
    if (url) {
      this.description = url;
    }
  }
}
