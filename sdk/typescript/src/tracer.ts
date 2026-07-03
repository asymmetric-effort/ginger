export const enum SpanKind {
  INTERNAL = 1,
  SERVER = 2,
  CLIENT = 3,
  PRODUCER = 4,
  CONSUMER = 5,
}

export const enum StatusCode {
  UNSET = 0,
  OK = 1,
  ERROR = 2,
}

export interface SpanOptions {
  kind?: SpanKind;
  attributes?: Record<string, string | number | boolean>;
}

export interface SpanEvent {
  name: string;
  timestamp: number;
  attributes: Record<string, string>;
}

export class Span {
  readonly traceId: string;
  readonly spanId: string;
  readonly name: string;
  readonly kind: SpanKind;
  readonly startTimeNs: bigint;
  endTimeNs: bigint = 0n;
  statusCode: StatusCode = StatusCode.UNSET;
  statusMessage = '';
  readonly attributes: Map<string, string | number | boolean> = new Map();
  readonly events: SpanEvent[] = [];
  private ended = false;

  constructor(traceId: string, spanId: string, name: string, kind: SpanKind) {
    this.traceId = traceId;
    this.spanId = spanId;
    this.name = name;
    this.kind = kind;
    this.startTimeNs = BigInt(Date.now()) * 1_000_000n;
  }

  setAttributes(attrs: Record<string, string | number | boolean>): void {
    for (const [k, v] of Object.entries(attrs)) {
      this.attributes.set(k, v);
    }
  }

  addEvent(name: string, attributes: Record<string, string> = {}): void {
    this.events.push({ name, timestamp: Date.now(), attributes });
  }

  recordException(error: Error): void {
    this.addEvent('exception', {
      'exception.type': error.name,
      'exception.message': error.message,
    });
  }

  setStatus(code: StatusCode, message = ''): void {
    this.statusCode = code;
    this.statusMessage = message;
  }

  end(): void {
    if (!this.ended) {
      this.ended = true;
      this.endTimeNs = BigInt(Date.now()) * 1_000_000n;
    }
  }
}

export interface TracerConfig {
  endpoint?: string;
  service?: string;
  samplingRate?: number;
}

export class Tracer {
  private readonly config: Required<TracerConfig>;

  constructor(config: TracerConfig = {}) {
    this.config = {
      endpoint: config.endpoint ?? 'http://localhost:4318',
      service: config.service ?? '',
      samplingRate: config.samplingRate ?? 1.0,
    };
  }

  startSpan(name: string, options: SpanOptions = {}): Span {
    const traceId = generateId(32);
    const spanId = generateId(16);
    const span = new Span(traceId, spanId, name, options.kind ?? SpanKind.INTERNAL);
    if (options.attributes) {
      span.setAttributes(options.attributes);
    }
    return span;
  }
}

function generateId(length: number): string {
  const chars = '0123456789abcdef';
  let result = '';
  for (let i = 0; i < length; i++) {
    result += chars[Math.floor(Math.random() * 16)];
  }
  return result;
}
