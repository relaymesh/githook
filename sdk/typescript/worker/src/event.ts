import { Buffer } from "node:buffer";

export interface Event {
  provider: string;
  type: string;
  topic: string;
  metadata: Record<string, string>;
  payload: Buffer;
  normalized?: Record<string, unknown>;
  requestId?: string;
  installationId?: string;
  logId?: string;
  client?: unknown;
}
