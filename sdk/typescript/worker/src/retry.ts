import type { WorkerContext } from "./context.js";
import type { Event } from "./event.js";

export interface RetryDecision {
  retry?: boolean;
  nack?: boolean;
  Retry?: boolean;
  Nack?: boolean;
}

export interface RetryPolicy {
  onError?: (ctx: WorkerContext, evt: Event | undefined, err: Error) => RetryDecision;
  OnError?: (ctx: WorkerContext, evt: Event | undefined, err: Error) => RetryDecision;
}

export class NoRetry implements RetryPolicy {
  onError(_ctx: WorkerContext, _evt: Event | undefined, _err: Error): RetryDecision {
    return { retry: false, nack: true };
  }
}

export function normalizeRetryDecision(decision: RetryDecision): { retry: boolean; nack: boolean } {
  return {
    retry: Boolean(decision.retry ?? decision.Retry),
    nack: Boolean(decision.nack ?? decision.Nack),
  };
}
