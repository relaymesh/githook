import assert from "node:assert/strict";
import test from "node:test";

import { Worker } from "../dist/worker.js";

class TestCodec {
  constructor(evt, err) {
    this.evt = evt;
    this.err = err;
  }

  decode() {
    if (this.err) {
      throw this.err;
    }
    return this.evt;
  }
}

test("handleMessage success updates status and skips retry", async () => {
  const retryCalls = [];
  const updates = [];
  const finish = [];

  const evt = {
    provider: "github",
    type: "push",
    topic: "topic",
    metadata: {},
    payload: Buffer.from("{}"),
  };
  const worker = new Worker({
    codec: new TestCodec(evt),
    retry: {
      onError(_ctx, _evt, err) {
        retryCalls.push(err);
        return { retry: false, nack: true };
      },
    },
    listeners: [
      {
        onMessageFinish(_ctx, _evt, err) {
          finish.push(err);
        },
      },
    ],
    logger: { printf() {} },
  });
  worker.eventLogsClient = () => ({
    updateStatus: async (logId, status, errorMessage) => {
      updates.push({ logId, status, errorMessage });
    },
  });
  worker.topicHandlers.set("topic", async () => {});

  const shouldNack = await worker.handleMessage(
    { tenantId: "acme" },
    "topic",
    { payload: Buffer.from("{}"), metadata: { log_id: "log-1" } },
  );

  assert.equal(shouldNack, false);
  assert.equal(retryCalls.length, 0);
  assert.deepEqual(finish, [undefined]);
  assert.deepEqual(updates, [
    { logId: "log-1", status: "success", errorMessage: undefined },
  ]);
});

test("handleMessage handler failure updates failed status and triggers retry", async () => {
  const retryCalls = [];
  const errors = [];
  const updates = [];

  const evt = {
    provider: "github",
    type: "push",
    topic: "topic",
    metadata: {},
    payload: Buffer.from("{}"),
  };
  const handlerErr = new Error("boom");

  const worker = new Worker({
    codec: new TestCodec(evt),
    retry: {
      onError(_ctx, _evt, err) {
        retryCalls.push(err);
        return { retry: false, nack: true };
      },
    },
    listeners: [
      {
        onError(_ctx, eventArg, err) {
          errors.push({ eventArg, err });
        },
      },
    ],
    logger: { printf() {} },
  });
  worker.eventLogsClient = () => ({
    updateStatus: async (logId, status, errorMessage) => {
      updates.push({ logId, status, errorMessage });
    },
  });
  worker.topicHandlers.set("topic", async () => {
    throw handlerErr;
  });

  const shouldNack = await worker.handleMessage(
    { tenantId: "acme" },
    "topic",
    { payload: Buffer.from("{}"), metadata: { log_id: "log-2" } },
  );

  assert.equal(shouldNack, true);
  assert.equal(retryCalls.length, 1);
  assert.equal(retryCalls[0], handlerErr);
  assert.equal(errors.length, 1);
  assert.equal(errors[0].eventArg?.type, "push");
  assert.equal(errors[0].err, handlerErr);
  assert.deepEqual(updates, [
    { logId: "log-2", status: "failed", errorMessage: "boom" },
  ]);
});

test("handleMessage decode failure marks failed with undefined event", async () => {
  const decodeErr = new Error("decode failed");
  const retryCalls = [];
  const errors = [];
  const updates = [];

  const worker = new Worker({
    codec: new TestCodec(undefined, decodeErr),
    retry: {
      onError(_ctx, evtArg, err) {
        retryCalls.push({ evtArg, err });
        return { retry: true, nack: false };
      },
    },
    listeners: [
      {
        onError(_ctx, eventArg, err) {
          errors.push({ eventArg, err });
        },
      },
    ],
    logger: { printf() {} },
  });
  worker.eventLogsClient = () => ({
    updateStatus: async (logId, status, errorMessage) => {
      updates.push({ logId, status, errorMessage });
    },
  });

  const shouldNack = await worker.handleMessage(
    { tenantId: "acme" },
    "topic",
    { payload: Buffer.from("{}"), metadata: { log_id: "log-3" } },
  );

  assert.equal(shouldNack, true);
  assert.equal(errors.length, 1);
  assert.equal(errors[0].eventArg, undefined);
  assert.equal(errors[0].err, decodeErr);
  assert.equal(retryCalls.length, 1);
  assert.equal(retryCalls[0].evtArg, undefined);
  assert.equal(retryCalls[0].err, decodeErr);
  assert.deepEqual(updates, [
    { logId: "log-3", status: "failed", errorMessage: "decode failed" },
  ]);
});
