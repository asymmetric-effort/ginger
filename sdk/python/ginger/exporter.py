"""OTLP HTTP exporter for the Ginger Python SDK."""

import json
import threading
import urllib.request
from typing import Optional

from ginger.tracer import Span


class OTLPHTTPExporter:
    """Exports spans to an OTLP-compatible endpoint via HTTP POST."""

    def __init__(
        self,
        endpoint: str = "http://localhost:4318",
        service: str = "unknown",
        batch_size: int = 512,
        timeout: float = 10.0,
    ) -> None:
        self.endpoint = endpoint.rstrip("/")
        self.service = service
        self.batch_size = batch_size
        self.timeout = timeout
        self._lock = threading.Lock()
        self._pending: list[Span] = []

    def export(self, *spans: Span) -> None:
        """Add spans and flush when the batch is full."""
        with self._lock:
            self._pending.extend(spans)
            should_flush = len(self._pending) >= self.batch_size

        if should_flush:
            self.flush()

    def flush(self) -> None:
        """Send all pending spans to the OTLP endpoint."""
        with self._lock:
            if not self._pending:
                return
            spans = self._pending
            self._pending = []

        payload = self._build_payload(spans)
        data = json.dumps(payload).encode("utf-8")
        url = self.endpoint + "/v1/traces"

        req = urllib.request.Request(
            url,
            data=data,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        with urllib.request.urlopen(req, timeout=self.timeout) as resp:
            if resp.status < 200 or resp.status >= 300:
                raise RuntimeError(f"OTLP export failed with status {resp.status}")

    def shutdown(self) -> None:
        """Flush any remaining spans."""
        self.flush()

    def _build_payload(self, spans: list[Span]) -> dict:
        """Build OTLP JSON trace export request."""
        otlp_spans = []
        for s in spans:
            otlp_span = {
                "traceId": s.trace_id,
                "spanId": s.span_id,
                "name": s.name,
                "kind": int(s.kind),
                "startTimeUnixNano": str(s.start_time_ns),
                "endTimeUnixNano": str(s.end_time_ns),
                "attributes": self._build_attributes(s.attributes),
                "status": self._build_status(s.status_code, s.status_message),
                "events": self._build_events(s.events),
            }
            otlp_spans.append(otlp_span)

        return {
            "resourceSpans": [
                {
                    "resource": {
                        "attributes": [
                            {
                                "key": "service.name",
                                "value": {"stringValue": self.service},
                            }
                        ]
                    },
                    "scopeSpans": [
                        {
                            "scope": {"name": "ginger-python"},
                            "spans": otlp_spans,
                        }
                    ],
                }
            ]
        }

    @staticmethod
    def _build_attributes(attrs: dict) -> list[dict]:
        """Convert attributes dict to OTLP attribute list."""
        result = []
        for k, v in attrs.items():
            if isinstance(v, bool):
                value = {"boolValue": v}
            elif isinstance(v, int):
                value = {"intValue": str(v)}
            elif isinstance(v, float):
                value = {"doubleValue": v}
            else:
                value = {"stringValue": str(v)}
            result.append({"key": k, "value": value})
        return result

    @staticmethod
    def _build_status(code: int, message: str) -> dict:
        """Build OTLP status object."""
        status: dict = {"code": int(code)}
        if message:
            status["message"] = message
        return status

    @staticmethod
    def _build_events(events: list[dict]) -> list[dict]:
        """Convert event list to OTLP event format."""
        result = []
        for ev in events:
            attrs = []
            for k, v in ev.get("attributes", {}).items():
                attrs.append({
                    "key": k,
                    "value": {"stringValue": str(v)},
                })
            result.append({
                "name": ev["name"],
                "timeUnixNano": str(ev["timestamp"]),
                "attributes": attrs,
            })
        return result
