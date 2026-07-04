"""Tracer implementation for the Ginger Python SDK."""

import os
import time
import uuid
from enum import IntEnum
from typing import Any


class SpanKind(IntEnum):
    INTERNAL = 1
    SERVER = 2
    CLIENT = 3
    PRODUCER = 4
    CONSUMER = 5


class StatusCode(IntEnum):
    UNSET = 0
    OK = 1
    ERROR = 2


class Span:
    """Represents a single traced operation."""

    def __init__(
        self,
        trace_id: str,
        span_id: str,
        name: str,
        kind: SpanKind = SpanKind.INTERNAL,
    ) -> None:
        self.trace_id = trace_id
        self.span_id = span_id
        self.name = name
        self.kind = kind
        self.start_time_ns = time.time_ns()
        self.end_time_ns: int = 0
        self.status_code = StatusCode.UNSET
        self.status_message = ""
        self.attributes: dict[str, Any] = {}
        self.events: list[dict[str, Any]] = []
        self._ended = False

    def set_attribute(self, key: str, value: Any) -> None:
        self.attributes[key] = value

    def set_status(self, code: StatusCode, message: str = "") -> None:
        self.status_code = code
        self.status_message = message

    def add_event(self, name: str, attributes: dict[str, str] | None = None) -> None:
        self.events.append(
            {"name": name, "timestamp": time.time_ns(), "attributes": attributes or {}}
        )

    def record_exception(self, exc: BaseException) -> None:
        self.add_event(
            "exception",
            {"exception.type": type(exc).__name__, "exception.message": str(exc)},
        )

    def end(self) -> None:
        if not self._ended:
            self._ended = True
            self.end_time_ns = time.time_ns()

    def __enter__(self) -> "Span":
        return self

    def __exit__(self, *args: Any) -> None:
        self.end()


class Tracer:
    """Creates and manages spans."""

    def __init__(
        self,
        endpoint: str = "http://localhost:4318",
        service: str = "",
        sampling_rate: float = 1.0,
    ) -> None:
        self.endpoint = endpoint
        self.service = service or os.environ.get("OTEL_SERVICE_NAME", "unknown")
        self.sampling_rate = sampling_rate

    def start_span(
        self,
        name: str,
        kind: SpanKind = SpanKind.INTERNAL,
        attributes: dict[str, Any] | None = None,
    ) -> Span:
        trace_id = uuid.uuid4().hex
        span_id = uuid.uuid4().hex[:16]
        span = Span(trace_id, span_id, name, kind)
        if attributes:
            for k, v in attributes.items():
                span.set_attribute(k, v)
        return span
