import os
import signal
import threading

from relaymesh_githook import New, WithEndpoint

stop = threading.Event()


def shutdown(_signum, _frame):
    stop.set()


signal.signal(signal.SIGINT, shutdown)
signal.signal(signal.SIGTERM, shutdown)

endpoint = os.getenv("GITHOOK_ENDPOINT", "https://relaymesh.vercel.app/api/connect")
rule_id = os.getenv("GITHOOK_RULE_ID", "85101e9f-3bcf-4ed0-b561-750c270ef6c3")

wk = New(
    WithEndpoint(endpoint),
)


def handle(ctx, evt):
    print(f"topic={evt.topic} provider={evt.provider} type={evt.type}")


wk.HandleRule(rule_id, handle)

wk.Run(stop)
