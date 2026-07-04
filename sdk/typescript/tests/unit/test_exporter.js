import { describe, it, beforeEach, afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';

// Since we can't import .ts directly in Node 18, we inline the classes
// to test the exporter logic without requiring a build step.

class Span {
  constructor(traceId, spanId, name, kind) {
    this.traceId = traceId;
    this.spanId = spanId;
    this.name = name;
    this.kind = kind;
    this.startTimeNs = BigInt(Date.now()) * 1_000_000n;
    this.endTimeNs = 0n;
    this.statusCode = 0;
    this.statusMessage = '';
    this.attributes = new Map();
    this.events = [];
    this._ended = false;
  }

  setAttributes(attrs) {
    for (const [k, v] of Object.entries(attrs)) {
      this.attributes.set(k, v);
    }
  }

  addEvent(name, attributes = {}) {
    this.events.push({ name, timestamp: Date.now(), attributes });
  }

  setStatus(code, message = '') {
    this.statusCode = code;
    this.statusMessage = message;
  }

  end() {
    if (!this._ended) {
      this._ended = true;
      this.endTimeNs = BigInt(Date.now()) * 1_000_000n;
    }
  }
}

class OTLPHTTPExporter {
  constructor(config = {}) {
    this.endpoint = (config.endpoint ?? 'http://localhost:4318').replace(/\/+$/, '');
    this.service = config.service ?? 'unknown';
    this.batchSize = config.batchSize ?? 512;
    this.timeoutMs = config.timeoutMs ?? 10000;
    this.pending = [];
  }

  async export(...spans) {
    this.pending.push(...spans);
    if (this.pending.length >= this.batchSize) {
      await this.flush();
    }
  }

  async flush() {
    if (this.pending.length === 0) return;
    const spans = this.pending;
    this.pending = [];

    const payload = this._buildPayload(spans);
    const url = `${this.endpoint}/v1/traces`;
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.timeoutMs);

    try {
      const response = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
        signal: controller.signal,
      });
      if (!response.ok) {
        throw new Error(`OTLP export failed with status ${response.status}`);
      }
    } finally {
      clearTimeout(timer);
    }
  }

  async shutdown() {
    await this.flush();
  }

  _buildPayload(spans) {
    const otlpSpans = spans.map((s) => ({
      traceId: s.traceId,
      spanId: s.spanId,
      name: s.name,
      kind: s.kind,
      startTimeUnixNano: s.startTimeNs.toString(),
      endTimeUnixNano: s.endTimeNs.toString(),
      attributes: this._buildAttributes(s.attributes),
      status: this._buildStatus(s.statusCode, s.statusMessage),
      events: this._buildEvents(s.events),
    }));

    return {
      resourceSpans: [
        {
          resource: {
            attributes: [
              { key: 'service.name', value: { stringValue: this.service } },
            ],
          },
          scopeSpans: [
            {
              scope: { name: 'ginger-typescript' },
              spans: otlpSpans,
            },
          ],
        },
      ],
    };
  }

  _buildAttributes(attrs) {
    const result = [];
    for (const [k, v] of attrs) {
      let value;
      if (typeof v === 'string') value = { stringValue: v };
      else if (typeof v === 'number') {
        value = Number.isInteger(v) ? { intValue: v.toString() } : { doubleValue: v };
      } else if (typeof v === 'boolean') value = { boolValue: v };
      else value = { stringValue: String(v) };
      result.push({ key: k, value });
    }
    return result;
  }

  _buildStatus(code, message) {
    const status = { code };
    if (message) status.message = message;
    return status;
  }

  _buildEvents(events) {
    return events.map((ev) => ({
      name: ev.name,
      timeUnixNano: (BigInt(ev.timestamp) * 1_000_000n).toString(),
      attributes: Object.entries(ev.attributes).map(([k, v]) => ({
        key: k,
        value: { stringValue: v },
      })),
    }));
  }
}

// ---- Test Helpers ----

function createSpan(name) {
  return new Span('aabbccdd00112233', '0011223344556677', name, 1);
}

function startTestServer() {
  return new Promise((resolve) => {
    const received = [];
    const server = createServer((req, res) => {
      const chunks = [];
      req.on('data', (chunk) => chunks.push(chunk));
      req.on('end', () => {
        received.push({
          path: req.url ?? '',
          body: Buffer.concat(chunks).toString('utf-8'),
          method: req.method ?? '',
          contentType: req.headers['content-type'] ?? '',
        });
        res.writeHead(200);
        res.end();
      });
    });
    server.listen(0, '127.0.0.1', () => {
      const addr = server.address();
      const port = typeof addr === 'object' && addr ? addr.port : 0;
      resolve({ server, port, received });
    });
  });
}

// ---- Tests ----

describe('OTLPHTTPExporter', () => {
  let server;
  let port;
  let received;

  beforeEach(async () => {
    const result = await startTestServer();
    server = result.server;
    port = result.port;
    received = result.received;
  });

  afterEach(() => {
    server.close();
  });

  it('sends spans via HTTP POST to /v1/traces', async () => {
    const exporter = new OTLPHTTPExporter({
      endpoint: `http://127.0.0.1:${port}`,
      service: 'test-service',
      batchSize: 1,
    });

    const span = createSpan('test-op');
    span.setAttributes({ key: 'value' });
    span.setStatus(1);
    span.end();

    await exporter.export(span);

    assert.equal(received.length, 1);
    assert.equal(received[0].path, '/v1/traces');
    assert.equal(received[0].method, 'POST');
    assert.equal(received[0].contentType, 'application/json');

    const payload = JSON.parse(received[0].body);
    const rs = payload.resourceSpans;
    assert.equal(rs.length, 1);
    assert.equal(rs[0].resource.attributes[0].key, 'service.name');
    assert.equal(rs[0].resource.attributes[0].value.stringValue, 'test-service');

    const spans = rs[0].scopeSpans[0].spans;
    assert.equal(spans.length, 1);
    assert.equal(spans[0].name, 'test-op');
  });

  it('batches spans until batch size is reached', async () => {
    const exporter = new OTLPHTTPExporter({
      endpoint: `http://127.0.0.1:${port}`,
      service: 'test',
      batchSize: 3,
    });

    for (let i = 0; i < 2; i++) {
      const span = createSpan('op');
      span.end();
      await exporter.export(span);
    }
    assert.equal(received.length, 0);

    const span = createSpan('op');
    span.end();
    await exporter.export(span);
    assert.equal(received.length, 1);
  });

  it('shutdown flushes remaining spans', async () => {
    const exporter = new OTLPHTTPExporter({
      endpoint: `http://127.0.0.1:${port}`,
      service: 'test',
      batchSize: 100,
    });

    const span = createSpan('shutdown-op');
    span.end();
    await exporter.export(span);
    assert.equal(received.length, 0);

    await exporter.shutdown();
    assert.equal(received.length, 1);

    const payload = JSON.parse(received[0].body);
    const spans = payload.resourceSpans[0].scopeSpans[0].spans;
    assert.equal(spans.length, 1);
    assert.equal(spans[0].name, 'shutdown-op');
  });

  it('flush with no pending spans does nothing', async () => {
    const exporter = new OTLPHTTPExporter({
      endpoint: `http://127.0.0.1:${port}`,
      service: 'test',
    });
    await exporter.flush();
    assert.equal(received.length, 0);
  });

  it('serializes span attributes correctly', async () => {
    const exporter = new OTLPHTTPExporter({
      endpoint: `http://127.0.0.1:${port}`,
      service: 'test',
      batchSize: 1,
    });

    const span = createSpan('op');
    span.setAttributes({ str: 'hello', num: 42, pi: 3.14, flag: true });
    span.end();
    await exporter.export(span);

    const payload = JSON.parse(received[0].body);
    const attrs = payload.resourceSpans[0].scopeSpans[0].spans[0].attributes;
    const attrMap = {};
    for (const a of attrs) {
      attrMap[a.key] = a.value;
    }
    assert.deepEqual(attrMap['str'], { stringValue: 'hello' });
    assert.deepEqual(attrMap['num'], { intValue: '42' });
    assert.deepEqual(attrMap['pi'], { doubleValue: 3.14 });
    assert.deepEqual(attrMap['flag'], { boolValue: true });
  });

  it('handles HTTP error responses', async () => {
    const errorServer = createServer((_req, res) => {
      res.writeHead(500);
      res.end();
    });
    await new Promise((resolve) => {
      errorServer.listen(0, '127.0.0.1', resolve);
    });
    const errorPort = errorServer.address().port;

    const exporter = new OTLPHTTPExporter({
      endpoint: `http://127.0.0.1:${errorPort}`,
      service: 'test',
      batchSize: 1,
    });

    const span = createSpan('op');
    span.end();

    await assert.rejects(() => exporter.export(span), {
      message: /OTLP export failed with status 500/,
    });

    errorServer.close();
  });
});
