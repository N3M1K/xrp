import * as vscode from 'vscode';
import { fetchProcesses, Process } from './socket';

export class HaloProvider implements vscode.TreeDataProvider<HaloTreeItem> {
  private _onDidChangeTreeData: vscode.EventEmitter<HaloTreeItem | undefined | void> = new vscode.EventEmitter<HaloTreeItem | undefined | void>();
  readonly onDidChangeTreeData: vscode.Event<HaloTreeItem | undefined | void> = this._onDidChangeTreeData.event;

  refresh(): void {
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(element: HaloTreeItem): vscode.TreeItem {
    return element;
  }

  async getChildren(element?: HaloTreeItem): Promise<HaloTreeItem[]> {
    if (element) {
      return [];
    }

    const processes = await fetchProcesses();
    if (processes.length === 0) {
      return [new HaloTreeItem("No running services detected.", "", "", vscode.TreeItemCollapsibleState.None)];
    }

    return processes.map(p => {
      const url = p.URL || (p.ProjectName ? `https://${p.ProjectName}.localhost` : `http://localhost:${p.Port}`);
      const label = p.ProjectName ? `${p.ProjectName} (${p.KnownApp || 'Unknown'})` : `${p.ProcessName}:${p.Port}`;
      const tooltip = p.TunnelURL
        ? `Port: ${p.Port} | PID: ${p.PID}\nPublic: ${p.TunnelURL}`
        : `Port: ${p.Port} | PID: ${p.PID}`;

      const item = new HaloTreeItem(
        label,
        url,
        tooltip,
        vscode.TreeItemCollapsibleState.None
      );

      // We set contextValue to halo-service so our package.json knows when to show the inline open icon
      if (p.ProjectName || p.Port) {
        item.contextValue = "halo-service";
      }
      return item;
    });
  }
}

export class HaloTreeItem extends vscode.TreeItem {
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
