"""Tests for the Ginger Python SDK tracer."""

import unittest

from ginger.tracer import SpanKind, StatusCode, Tracer


class TestTracer(unittest.TestCase):
    def test_start_span(self) -> None:
        tracer = Tracer(service="test")
        span = tracer.start_span("operation")
        self.assertEqual(span.name, "operation")
        self.assertFalse(span._ended)
        span.end()
        self.assertTrue(span._ended)
        self.assertGreater(span.end_time_ns, 0)

    def test_span_context_manager(self) -> None:
        tracer = Tracer(service="test")
        with tracer.start_span("op") as span:
            span.set_attribute("key", "value")
        self.assertTrue(span._ended)
        self.assertEqual(span.attributes["key"], "value")

    def test_span_set_status(self) -> None:
        tracer = Tracer(service="test")
        span = tracer.start_span("op")
        span.set_status(StatusCode.ERROR, "failed")
        self.assertEqual(span.status_code, StatusCode.ERROR)
        span.end()

    def test_span_record_exception(self) -> None:
        tracer = Tracer(service="test")
        span = tracer.start_span("op")
        span.record_exception(ValueError("bad value"))
        self.assertEqual(len(span.events), 1)
        self.assertEqual(span.events[0]["name"], "exception")
        span.end()

    def test_span_add_event(self) -> None:
        tracer = Tracer(service="test")
        span = tracer.start_span("op")
        span.add_event("checkpoint", {"step": "1"})
        self.assertEqual(len(span.events), 1)
        span.end()

    def test_span_kind(self) -> None:
        tracer = Tracer(service="test")
        span = tracer.start_span("op", kind=SpanKind.SERVER)
        self.assertEqual(span.kind, SpanKind.SERVER)
        span.end()

    def test_span_with_attributes(self) -> None:
        tracer = Tracer(service="test")
        span = tracer.start_span("op", attributes={"env": "prod"})
        self.assertEqual(span.attributes["env"], "prod")
        span.end()

    def test_double_end(self) -> None:
        tracer = Tracer(service="test")
        span = tracer.start_span("op")
        span.end()
        first_end = span.end_time_ns
        span.end()  # should be no-op
        self.assertEqual(span.end_time_ns, first_end)


if __name__ == "__main__":
    unittest.main()
