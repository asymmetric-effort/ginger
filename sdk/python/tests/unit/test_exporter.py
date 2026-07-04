"""Tests for the OTLP HTTP exporter."""

import json
import threading
import unittest
from http.server import HTTPServer, BaseHTTPRequestHandler

from ginger.exporter import OTLPHTTPExporter
from ginger.tracer import Tracer, StatusCode, SpanKind


class _OTLPHandler(BaseHTTPRequestHandler):
    """Test HTTP handler that records received payloads."""

    received: list[bytes] = []
    lock = threading.Lock()

    def do_POST(self) -> None:
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length)
        with self.lock:
            self.received.append(body)
        self.send_response(200)
        self.end_headers()

    def log_message(self, format: str, *args: object) -> None:
        pass  # suppress logs during tests


class TestOTLPHTTPExporter(unittest.TestCase):
    def setUp(self) -> None:
        _OTLPHandler.received = []
        self.server = HTTPServer(("127.0.0.1", 0), _OTLPHandler)
        self.port = self.server.server_address[1]
        self.thread = threading.Thread(target=self.server.serve_forever)
        self.thread.daemon = True
        self.thread.start()
        self.endpoint = f"http://127.0.0.1:{self.port}"

    def tearDown(self) -> None:
        self.server.shutdown()
        self.thread.join(timeout=5)

    def test_export_sends_spans(self) -> None:
        tracer = Tracer(service="test")
        span = tracer.start_span("test-op")
        span.set_attribute("key", "value")
        span.set_status(StatusCode.OK)
        span.end()

        exporter = OTLPHTTPExporter(
            endpoint=self.endpoint, service="test-service", batch_size=1
        )
        exporter.export(span)

        self.assertEqual(len(_OTLPHandler.received), 1)
        payload = json.loads(_OTLPHandler.received[0])

        rs = payload["resourceSpans"]
        self.assertEqual(len(rs), 1)

        resource = rs[0]["resource"]
        attrs = resource["attributes"]
        self.assertEqual(attrs[0]["key"], "service.name")
        self.assertEqual(attrs[0]["value"]["stringValue"], "test-service")

        scope_spans = rs[0]["scopeSpans"]
        spans = scope_spans[0]["spans"]
        self.assertEqual(len(spans), 1)
        self.assertEqual(spans[0]["name"], "test-op")

    def test_batching(self) -> None:
        tracer = Tracer(service="test")
        exporter = OTLPHTTPExporter(
            endpoint=self.endpoint, service="test", batch_size=3
        )

        for _ in range(2):
            span = tracer.start_span("op")
            span.end()
            exporter.export(span)

        self.assertEqual(len(_OTLPHandler.received), 0)

        span = tracer.start_span("op")
        span.end()
        exporter.export(span)

        self.assertEqual(len(_OTLPHandler.received), 1)

    def test_shutdown_flushes(self) -> None:
        tracer = Tracer(service="test")
        exporter = OTLPHTTPExporter(
            endpoint=self.endpoint, service="test", batch_size=100
        )

        span = tracer.start_span("shutdown-op")
        span.end()
        exporter.export(span)

        self.assertEqual(len(_OTLPHandler.received), 0)

        exporter.shutdown()

        self.assertEqual(len(_OTLPHandler.received), 1)
        payload = json.loads(_OTLPHandler.received[0])
        spans = payload["resourceSpans"][0]["scopeSpans"][0]["spans"]
        self.assertEqual(len(spans), 1)

    def test_flush_empty(self) -> None:
        exporter = OTLPHTTPExporter(
            endpoint=self.endpoint, service="test", batch_size=10
        )
        exporter.flush()  # should not raise
        self.assertEqual(len(_OTLPHandler.received), 0)

    def test_span_attributes_serialized(self) -> None:
        tracer = Tracer(service="test")
        span = tracer.start_span("op")
        span.set_attribute("str_key", "hello")
        span.set_attribute("int_key", 42)
        span.set_attribute("float_key", 3.14)
        span.set_attribute("bool_key", True)
        span.end()

        exporter = OTLPHTTPExporter(
            endpoint=self.endpoint, service="test", batch_size=1
        )
        exporter.export(span)

        payload = json.loads(_OTLPHandler.received[0])
        attrs = payload["resourceSpans"][0]["scopeSpans"][0]["spans"][0]["attributes"]
        attr_map = {a["key"]: a["value"] for a in attrs}
        self.assertEqual(attr_map["str_key"], {"stringValue": "hello"})
        self.assertEqual(attr_map["int_key"], {"intValue": "42"})
        self.assertEqual(attr_map["float_key"], {"doubleValue": 3.14})
        self.assertEqual(attr_map["bool_key"], {"boolValue": True})

    def test_span_events_serialized(self) -> None:
        tracer = Tracer(service="test")
        span = tracer.start_span("op")
        span.add_event("checkpoint", {"step": "1"})
        span.record_exception(ValueError("bad"))
        span.end()

        exporter = OTLPHTTPExporter(
            endpoint=self.endpoint, service="test", batch_size=1
        )
        exporter.export(span)

        payload = json.loads(_OTLPHandler.received[0])
        events = payload["resourceSpans"][0]["scopeSpans"][0]["spans"][0]["events"]
        self.assertEqual(len(events), 2)
        self.assertEqual(events[0]["name"], "checkpoint")
        self.assertEqual(events[1]["name"], "exception")

    def test_span_kind_serialized(self) -> None:
        tracer = Tracer(service="test")
        span = tracer.start_span("op", kind=SpanKind.SERVER)
        span.end()

        exporter = OTLPHTTPExporter(
            endpoint=self.endpoint, service="test", batch_size=1
        )
        exporter.export(span)

        payload = json.loads(_OTLPHandler.received[0])
        otlp_span = payload["resourceSpans"][0]["scopeSpans"][0]["spans"][0]
        self.assertEqual(otlp_span["kind"], 2)


if __name__ == "__main__":
    unittest.main()
