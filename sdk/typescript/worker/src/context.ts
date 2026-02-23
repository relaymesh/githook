export interface WorkerContext {
  tenantId?: string;
  signal?: AbortSignal;
  topic?: string;
  requestId?: string;
  logId?: string;
}
