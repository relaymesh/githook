from typing import Any, Callable, Protocol

from .context import WorkerContext
from .event import Event


class ClientProvider(Protocol):
    def client(self, ctx: WorkerContext, evt: Event) -> Any:
        ...

    def Client(self, ctx: WorkerContext, evt: Event) -> Any:
        ...


def client_provider_func(fn: Callable[[WorkerContext, Event], Any]) -> ClientProvider:
    class _Provider:
        def client(self, ctx: WorkerContext, evt: Event) -> Any:
            return fn(ctx, evt)

        def Client(self, ctx: WorkerContext, evt: Event) -> Any:
            return fn(ctx, evt)

    return _Provider()


def ClientProviderFunc(fn: Callable[[WorkerContext, Event], Any]) -> ClientProvider:
    return client_provider_func(fn)
