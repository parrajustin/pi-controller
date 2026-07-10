import { getAppClock } from './clock-provider.js';

export class WSClient {
  private ws: WebSocket | null = null;
  public readonly url: string;
  private requestMap = new Map<string, { resolve: (val: any) => void; reject: (err: any) => void }>();
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
    this.ws = new WebSocket(this.url);

    this.ws.onopen = () => {
      console.log('WS Connected');
      this.request({ type: 'get_state', payload: {} }).then(state => {
        this.notifyListeners(state);
      }).catch(err => console.error("failed to get state", err));
    };

    this.ws.onmessage = (event) => {
      const data = JSON.parse(event.data);

      if (data.type === 'state_update') {
        this.notifyListeners(data.payload);
      } else if (data.type === 'response') {
        const handler = this.requestMap.get(data.id);
        if (handler) {
          if (data.error) {
            handler.reject(new Error(data.error));
          } else {
            handler.resolve(data.payload);
          }
          this.requestMap.delete(data.id);
        }
      }
    };

    this.ws.onclose = () => {
      console.log('WS Disconnected. Reconnecting in .5s...');
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

  public request(msg: { type: string; payload?: any }): Promise<any> {
    return new Promise((resolve, reject) => {
      if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
        return reject(new Error('WebSocket not connected'));
      }
      
      const id = String(++this.reqId);
      this.requestMap.set(id, { resolve, reject });

      this.ws.send(JSON.stringify({
        id,
        type: msg.type,
        payload: msg.payload || {}
      }));
    });
  }
}

export const wsClient = new WSClient();
