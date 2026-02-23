export interface RelaybusMessage {
  topic?: string;
  payload?: Uint8Array;
  metadata?: Record<string, string>;
  [key: string]: unknown;
}

export type MessageHandler = (msg: RelaybusMessage) => Promise<void> | void;
