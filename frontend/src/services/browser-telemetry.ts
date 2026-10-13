import { api } from './api';

type BrowserLogEvent = {
  source: 'console.error' | 'window.error' | 'unhandledrejection';
  error_type: string;
};

const builtInErrorTypes = new Set([
  'Error', 'EvalError', 'RangeError', 'ReferenceError', 'SyntaxError',
  'TypeError', 'URIError', 'AggregateError',
]);
const lastReported = new Map<string, number>();
let pending: BrowserLogEvent[] = [];
let flushTimer: number | undefined;

function safeErrorType(value: unknown, fallback: string): string {
  if (!(value instanceof Error)) return fallback;
  return builtInErrorTypes.has(value.name) ? value.name : 'Error';
}

function report(source: BrowserLogEvent['source'], errorType: string) {
  const now = Date.now();
  const key = `${source}:${errorType}`;
  if ((lastReported.get(key) ?? 0) > now - 60_000) return;
  // Bound both retained keys and outbound events if a page enters an error loop.
  if (pending.length >= 20) return;
  lastReported.set(key, now);
  if (lastReported.size > 30) {
    for (const [oldKey, time] of lastReported) {
      if (time <= now - 60_000) lastReported.delete(oldKey);
    }
  }
  pending.push({ source, error_type: errorType });
  if (flushTimer === undefined) {
    flushTimer = window.setTimeout(() => {
      const batch = pending;
      pending = [];
      flushTimer = undefined;
      void api.reportBrowserLogs(batch).catch(() => {
        // Telemetry failures must never create more telemetry or affect the app.
      });
    }, 1000);
  }
}

let installed = false;

export function installBrowserTelemetry() {
  if (installed || typeof window === 'undefined') return;
  installed = true;

  const originalConsoleError = console.error.bind(console);
  console.error = (...args: unknown[]) => {
    originalConsoleError(...args);
    const error = args.find((value): value is Error => value instanceof Error);
    report('console.error', safeErrorType(error, 'ConsoleError'));
  };

  window.addEventListener('error', (event) => {
    report('window.error', safeErrorType(event.error, 'Error'));
  });

  window.addEventListener('unhandledrejection', (event) => {
    report('unhandledrejection', safeErrorType(event.reason, 'NonErrorRejection'));
  });
}
