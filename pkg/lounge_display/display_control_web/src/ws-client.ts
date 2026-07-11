import { getAppClock } from './clock-provider.js';
import { Result, Ok, Err, StatusError, UnknownError, UnavailableError, Optional, None, Some } from 'standard-ts-lib/src/index.js';
import { tracer, meter } from './telemetry.js';
import { SpanStatusCode, context, propagation, trace } from '@opentelemetry/api';

const wsDuration = meter.createHistogram('websocket.request.duration', {
  description: 'Duration of websocket requests',
  unit: 'ms',
});

export class WSClient {
  private ws: Optional<WebSocket> = None;
  public readonly url: string;
  private requestMap = new Map<string, { resolve: (val: Result<any, StatusError>) => void }>();
  private reqId = 0;
  private listeners = new Set<(state: any) => void>();

  constructor(url?: string) {
    if (url) {
      this.url = url;
    } else {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const host = window.location.host || 'localhost';
      this.url = `${protocol}//${host}/ws`;
    }
    this.connect();
  }

  private connect() {
    const ws = new WebSocket(this.url);
    this.ws = Some(ws);

    ws.onopen = () => {
      console.log('WS Connected');
      this.request({ type: 'get_state', payload: {} }).then(res => {
        if (res.ok) {
          this.notifyListeners(res.safeUnwrap());
        } else {
          console.error("failed to get state", res.val);
        }
      });
    };

    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);

      if (data.type === 'state_update') {
        this.notifyListeners(data.payload);
      } else if (data.type === 'response') {
        const handler = this.requestMap.get(data.id);
        if (handler) {
          if (data.error) {
            handler.resolve(Err(UnknownError(data.error)));
          } else {
            handler.resolve(Ok(data.payload));
          }
          this.requestMap.delete(data.id);
        }
      }
    };

    ws.onclose = () => {
      console.log('WS Disconnected. Reconnecting in .5s...');
      this.ws = None;
      getAppClock().setTimeout(async () => { this.connect(); }, 500);
    };
  }

  public onStateUpdate(cb: (state: any) => void) {
    this.listeners.add(cb);
    return () => this.listeners.delete(cb);
  }

  private notifyListeners(state: any) {
    for (const listener of this.listeners) {
      listener(state);
    }
  }

  public request(msg: { type: string; payload?: any }): Promise<Result<any, StatusError>> {
    return new Promise((resolve) => {
      const span = tracer.startSpan(`websocket.request.${msg.type}`);
      const startTime = performance.now();

      if (this.ws.none || this.ws.safeValue().readyState !== WebSocket.OPEN) {
        span.setStatus({ code: SpanStatusCode.ERROR, message: 'WebSocket not connected' });
        span.end();
        return resolve(Err(UnavailableError('WebSocket not connected')));
      }
      
      const id = String(++this.reqId);

      const carrier: any = {};
      propagation.inject(trace.setSpan(context.active(), span), carrier);

      this.requestMap.set(id, { 
        resolve: (val) => {
          wsDuration.record(performance.now() - startTime, { type: msg.type });
          if (!val.ok) {
            span.setStatus({ code: SpanStatusCode.ERROR, message: val.val.message });
          } else {
            span.setStatus({ code: SpanStatusCode.OK });
          }
          span.end();
          resolve(val);
        }
      });

      this.ws.safeValue().send(JSON.stringify({
        id,
        type: msg.type,
        payload: msg.payload || {},
        traceparent: carrier.traceparent,
        tracestate: carrier.tracestate
      }));
    });
  }
}

export const wsClient = new WSClient();
