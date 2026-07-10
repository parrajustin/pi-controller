import { getAppClock } from './clock-provider.js';
import { Result, Ok, Err, StatusError, UnknownError, UnavailableError, Optional, None, Some } from 'standard-ts-lib/src/index.js';

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
      if (this.ws.none || this.ws.safeValue().readyState !== WebSocket.OPEN) {
        return resolve(Err(UnavailableError('WebSocket not connected')));
      }
      
      const id = String(++this.reqId);
      this.requestMap.set(id, { resolve });

      this.ws.safeValue().send(JSON.stringify({
        id,
        type: msg.type,
        payload: msg.payload || {}
      }));
    });
  }
}

export const wsClient = new WSClient();
