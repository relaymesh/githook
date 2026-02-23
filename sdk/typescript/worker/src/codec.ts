import { Buffer } from "node:buffer";

import { EventPayload } from "../gen/cloud/v1/githooks_pb.js";
import {
  MetadataKeyEvent,
  MetadataKeyInstallationID,
  MetadataKeyLogID,
  MetadataKeyProvider,
  MetadataKeyRequestID,
} from "./metadata.js";
import type { Event } from "./event.js";
import type { RelaybusMessage } from "./types.js";

export interface Codec {
  decode(topic: string | undefined, msg: RelaybusMessage): Event;
}

export class DefaultCodec implements Codec {
  decode(topic: string | undefined, msg: RelaybusMessage): Event {
    if (!msg || !msg.payload) {
      throw new Error("message payload is required");
    }

    const payloadBytes = toUint8Array(msg.payload);
    let provider = "";
    let eventName = "";
    let rawPayload = Buffer.from(payloadBytes);
    let normalized: Record<string, unknown> | undefined;

    try {
      const env = EventPayload.fromBinary(payloadBytes);
      provider = env.provider ?? "";
      eventName = env.name ?? "";
      rawPayload = Buffer.from(env.payload ?? new Uint8Array());
      normalized = parseJSONObject(rawPayload);
    } catch (err) {
      const legacy = parseJSONValue(payloadBytes);
      if (legacy && typeof legacy === "object") {
        if ("provider" in legacy && typeof legacy.provider === "string") {
          provider = legacy.provider;
        }
        if ("name" in legacy && typeof legacy.name === "string") {
          eventName = legacy.name;
        }
        if ("data" in legacy && legacy.data && typeof legacy.data === "object") {
          normalized = legacy.data as Record<string, unknown>;
        }
      }
      if (!normalized) {
        normalized = parseJSONObject(rawPayload);
      }
      if (!normalized && err instanceof Error) {
        throw err;
      }
    }

    const metadata: Record<string, string> = { ...(msg.metadata ?? {}) };
    if (!provider) {
      provider = metadata[MetadataKeyProvider] ?? "";
    }
    if (!eventName) {
      eventName = metadata[MetadataKeyEvent] ?? "";
    }

    return {
      provider,
      type: eventName,
      topic: resolveTopic(topic, msg),
      metadata,
      payload: rawPayload,
      normalized,
      requestId: metadata[MetadataKeyRequestID],
      installationId: metadata[MetadataKeyInstallationID],
      logId: metadata[MetadataKeyLogID],
    };
  }
}

function resolveTopic(topic: string | undefined, msg: RelaybusMessage): string {
  const trimmed = (topic ?? "").trim();
  if (trimmed) {
    return trimmed;
  }
  return (msg.topic ?? "").toString();
}

function parseJSONObject(data: Uint8Array): Record<string, unknown> | undefined {
  const value = parseJSONValue(data);
  if (value && typeof value === "object" && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  return undefined;
}

function parseJSONValue(data: Uint8Array): unknown {
  if (!data || data.length === 0) {
    return undefined;
  }
  try {
    return JSON.parse(Buffer.from(data).toString("utf8"));
  } catch {
    return undefined;
  }
}

function toUint8Array(input: Uint8Array): Uint8Array {
  if (input instanceof Uint8Array) {
    return input;
  }
  return new Uint8Array(input);
}
