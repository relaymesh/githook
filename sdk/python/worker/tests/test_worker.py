import unittest

from relaymesh_githook.context import WorkerContext
from relaymesh_githook.codec import Codec
from relaymesh_githook.event import Event
from relaymesh_githook.event_log_status import (
    EVENT_LOG_STATUS_FAILED,
    EVENT_LOG_STATUS_SUCCESS,
)
from relaymesh_githook.listener import Listener
from relaymesh_githook.retry import RetryPolicy
from relaymesh_githook.retry import RetryDecision
from relaymesh_githook.types import RelaybusMessage
from relaymesh_githook.worker import Worker, WorkerOptions


class TestCodec(Codec):
    def __init__(self, event=None, err=None):
        self._event = event
        self._err = err

    def decode(self, topic, msg):
        if self._err is not None:
            raise self._err
        if self._event is None:
            raise ValueError("event is required for TestCodec")
        return self._event


class TestRetryPolicy(RetryPolicy):
    def __init__(self, decision):
        self.decision = decision
        self.calls = 0
        self.last_evt = None
        self.last_err = None

    def on_error(self, ctx, evt, err):
        self.calls += 1
        self.last_evt = evt
        self.last_err = err
        return self.decision


class TestListener(Listener):
    def __init__(self):
        self.finish_calls = []
        self.error_calls = []

    def OnStart(self, ctx):
        return None

    def OnExit(self, ctx):
        return None

    def OnMessageStart(self, ctx, evt):
        return None

    def OnMessageFinish(self, ctx, evt, err=None):
        self.finish_calls.append(err)

    def OnError(self, ctx, evt, err):
        self.error_calls.append((evt, err))


class WorkerHandleMessageTests(unittest.TestCase):
    def test_handle_message_success_updates_success_status(self):
        event = Event(
            provider="github",
            type="push",
            topic="topic",
            metadata={},
            payload=b"{}",
        )
        retry = TestRetryPolicy(RetryDecision(retry=False, nack=True))
        listener = TestListener()
        worker = Worker(
            WorkerOptions(
                codec=TestCodec(event=event),
                retry=retry,
                listeners=[listener],
            )
        )
        status_calls = []

        def capture_status(ctx, log_id, status, err):
            status_calls.append((log_id, status, str(err or "")))

        worker.update_event_log_status = capture_status
        worker.topic_handlers["topic"] = lambda ctx, evt: None

        should_nack = worker.handle_message(
            WorkerContext(tenant_id="acme"),
            "topic",
            RelaybusMessage(topic="topic", payload=b"{}", metadata={"log_id": "log-1"}),
        )

        self.assertFalse(should_nack)
        self.assertEqual(retry.calls, 0)
        self.assertEqual(listener.finish_calls, [None])
        self.assertEqual(status_calls, [("log-1", EVENT_LOG_STATUS_SUCCESS, "")])

    def test_handle_message_handler_error_updates_failed_status(self):
        event = Event(
            provider="github",
            type="push",
            topic="topic",
            metadata={},
            payload=b"{}",
        )
        retry = TestRetryPolicy(RetryDecision(retry=False, nack=True))
        listener = TestListener()
        worker = Worker(
            WorkerOptions(
                codec=TestCodec(event=event),
                retry=retry,
                listeners=[listener],
            )
        )
        status_calls = []

        def capture_status(ctx, log_id, status, err):
            status_calls.append((log_id, status, str(err or "")))

        worker.update_event_log_status = capture_status

        def fail_handler(ctx, evt):
            raise RuntimeError("handler failed")

        worker.topic_handlers["topic"] = fail_handler

        should_nack = worker.handle_message(
            WorkerContext(tenant_id="acme"),
            "topic",
            RelaybusMessage(topic="topic", payload=b"{}", metadata={"log_id": "log-2"}),
        )

        self.assertTrue(should_nack)
        self.assertEqual(retry.calls, 1)
        self.assertIsNotNone(retry.last_evt)
        self.assertEqual(str(retry.last_err), "handler failed")
        self.assertEqual(len(listener.error_calls), 1)
        self.assertIsNotNone(listener.error_calls[0][0])
        self.assertEqual(str(listener.error_calls[0][1]), "handler failed")
        self.assertEqual(
            status_calls,
            [("log-2", EVENT_LOG_STATUS_FAILED, "handler failed")],
        )

    def test_handle_message_decode_error_uses_nil_event(self):
        retry = TestRetryPolicy(RetryDecision(retry=True, nack=False))
        listener = TestListener()
        worker = Worker(
            WorkerOptions(
                codec=TestCodec(err=ValueError("decode failed")),
                retry=retry,
                listeners=[listener],
            )
        )
        status_calls = []

        def capture_status(ctx, log_id, status, err):
            status_calls.append((log_id, status, str(err or "")))

        worker.update_event_log_status = capture_status

        should_nack = worker.handle_message(
            WorkerContext(tenant_id="acme"),
            "topic",
            RelaybusMessage(topic="topic", payload=b"{}", metadata={"log_id": "log-3"}),
        )

        self.assertTrue(should_nack)
        self.assertEqual(retry.calls, 1)
        self.assertIsNone(retry.last_evt)
        self.assertEqual(str(retry.last_err), "decode failed")
        self.assertEqual(len(listener.error_calls), 1)
        self.assertIsNone(listener.error_calls[0][0])
        self.assertEqual(str(listener.error_calls[0][1]), "decode failed")
        self.assertEqual(
            status_calls,
            [("log-3", EVENT_LOG_STATUS_FAILED, "decode failed")],
        )


if __name__ == "__main__":
    unittest.main()
