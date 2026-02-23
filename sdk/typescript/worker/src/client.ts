import type { WorkerContext } from "./context.js";
import type { Event } from "./event.js";

export interface ClientProvider {
  client?: (ctx: WorkerContext, evt: Event) => Promise<unknown> | unknown;
  Client?: (ctx: WorkerContext, evt: Event) => Promise<unknown> | unknown;
}

export type ClientProviderFunc = (ctx: WorkerContext, evt: Event) => Promise<unknown> | unknown;

export function clientProviderFunc(fn: ClientProviderFunc): ClientProvider {
  return {
    client: fn,
    Client: fn,
  };
}

export function ClientProviderFunc(fn: ClientProviderFunc): ClientProvider {
  return clientProviderFunc(fn);
}
