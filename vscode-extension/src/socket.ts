import * as net from 'net';
import * as os from 'os';
import * as path from 'path';

export interface Process {
  PID: number;
  Port: number;
  ProcessName: string;
  ProjectName: string;
  CWD: string;
  KnownApp: string;
}

export interface Response {
  success: boolean;
  data?: any;
  error?: string;
}

function getSocketPath(): string {
  return path.join(os.tmpdir(), "xrp.sock");
}

export async function sendCommand(cmd: string, args?: Record<string, string>): Promise<Response> {
  return new Promise((resolve, reject) => {
    const client = net.createConnection({ path: getSocketPath() });

    client.on('connect', () => {
      client.write(JSON.stringify({ cmd, args }) + '\n');
    });

    let buffer = '';
    client.on('data', (data) => {
      buffer += data.toString();
    });

    client.on('end', () => {
      try {
        const resp = JSON.parse(buffer) as Response;
        resolve(resp);
      } catch (err) {
        reject(err);
      }
    });

    client.on('error', (err) => {
      reject(err);
    });
  });
}

export async function fetchProcesses(): Promise<Process[]> {
  try {
    const resp = await sendCommand('list');
    if (resp.success) {
      if (typeof resp.data === 'string') {
        return JSON.parse(resp.data) as Process[];
      }
      return resp.data as Process[];
    }
  } catch (err) {
    // Daemon offline
  }
  return [];
}
