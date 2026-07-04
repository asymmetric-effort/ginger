import { Span, StatusCode } from './tracer.js';

/**
 * Configuration for the OTLP HTTP exporter.
 */
export interface OTLPExporterConfig {
  endpoint?: string;
  service?: string;
  batchSize?: number;
  timeoutMs?: number;
}

/**
 * Exports spans to an OTLP-compatible endpoint via HTTP POST using fetch.
 */
export class OTLPHTTPExporter {
  private readonly endpoint: string;
  private readonly service: string;
  private readonly batchSize: number;
  private readonly timeoutMs: number;
  private pending: Span[] = [];

  constructor(config: OTLPExporterConfig = {}) {
    this.endpoint = (config.endpoint ?? 'http://localhost:4318').replace(/\/+$/, '');
    this.service = config.service ?? 'unknown';
    this.batchSize = config.batchSize ?? 512;
    this.timeoutMs = config.timeoutMs ?? 10000;
  }

  /**
   * Add spans to the pending batch. Flushes when the batch is full.
   */
  async export(...spans: Span[]): Promise<void> {
    this.pending.push(...spans);
    if (this.pending.length >= this.batchSize) {
      await this.flush();
    }
  }

  /**
   * Send all pending spans to the OTLP endpoint.
   */
  async flush(): Promise<void> {
    if (this.pending.length === 0) {
      return;
    }

    const spans = this.pending;
    this.pending = [];

    const payload = this.buildPayload(spans);
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

  /**
   * Flush any remaining spans.
   */
  async shutdown(): Promise<void> {
    await this.flush();
  }

  private buildPayload(spans: Span[]): Record<string, unknown> {
    const otlpSpans = spans.map((s) => ({
      traceId: s.traceId,
      spanId: s.spanId,
      name: s.name,
      kind: s.kind as number,
      startTimeUnixNano: s.startTimeNs.toString(),
      endTimeUnixNano: s.endTimeNs.toString(),
      attributes: this.buildAttributes(s.attributes),
      status: this.buildStatus(s.statusCode, s.statusMessage),
      events: this.buildEvents(s.events),
    }));

    return {
      resourceSpans: [
        {
          resource: {
            attributes: [
              {
                key: 'service.name',
                value: { stringValue: this.service },
              },
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

  private buildAttributes(
    attrs: Map<string, string | number | boolean>,
  ): Array<Record<string, unknown>> {
    const result: Array<Record<string, unknown>> = [];
    for (const [k, v] of attrs) {
      let value: Record<string, unknown>;
      if (typeof v === 'string') {
        value = { stringValue: v };
      } else if (typeof v === 'number') {
        if (Number.isInteger(v)) {
          value = { intValue: v.toString() };
        } else {
          value = { doubleValue: v };
        }
      } else if (typeof v === 'boolean') {
        value = { boolValue: v };
      } else {
        value = { stringValue: String(v) };
      }
      result.push({ key: k, value });
    }
    return result;
  }

  private buildStatus(
    code: StatusCode,
    message: string,
  ): Record<string, unknown> {
    const status: Record<string, unknown> = { code: code as number };
    if (message) {
      status.message = message;
    }
    return status;
  }

  private buildEvents(
    events: Array<{ name: string; timestamp: number; attributes: Record<string, string> }>,
  ): Array<Record<string, unknown>> {
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
