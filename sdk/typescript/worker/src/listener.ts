import type { WorkerContext } from "./context.js";
import type { Event } from "./event.js";

export interface Listener {
  onStart?: (ctx: WorkerContext) => void;
  onExit?: (ctx: WorkerContext) => void;
  onMessageStart?: (ctx: WorkerContext, evt: Event) => void;
  onMessageFinish?: (ctx: WorkerContext, evt: Event, err?: Error) => void;
  onError?: (ctx: WorkerContext, evt: Event | undefined, err: Error) => void;
  OnStart?: (ctx: WorkerContext) => void;
  OnExit?: (ctx: WorkerContext) => void;
  OnMessageStart?: (ctx: WorkerContext, evt: Event) => void;
  OnMessageFinish?: (ctx: WorkerContext, evt: Event, err?: Error) => void;
  OnError?: (ctx: WorkerContext, evt: Event | undefined, err: Error) => void;
}
