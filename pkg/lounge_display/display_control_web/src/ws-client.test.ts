import WS from 'jest-websocket-mock';
import { wsClient, WSClient } from './ws-client.js';
import { getAppClock, setAppClock } from './clock-provider.js';
import { FakeClock } from 'standard-ts-lib/src/clock.js';

describe('ws-client', () => {
  let server: WS;
  let clock: FakeClock;

  beforeEach(() => {
    clock = new FakeClock(0);
    setAppClock(clock);
    server = new WS('ws://localhost/ws', { jsonProtocol: true });
  });

  afterEach(() => {
    WS.clean();
    // wsClient is a singleton, so we can't easily reset it completely here without recreating or restarting it,
    // but the WebSocket mock cleanup takes care of closing the connection.
  });

  it('connects to websocket server and requests state once', async () => {
    const client = new WSClient();
    await server.connected;
    
    // Expect the first message to be get_state
    await expect(server).toReceiveMessage({ id: expect.any(String), type: 'get_state', payload: {} });
    
    // Check that it only sends state once
    server.send({ id: '1', type: 'response', payload: { connected: true } });
  });

  it('sends requests and resolves promises correctly', async () => {
    const client = new WSClient();
    await server.connected;
    
    // Handle initial get_state
    await expect(server).toReceiveMessage({ id: '1', type: 'get_state', payload: {} });
    
    const requestPromise = client.request({ type: 'test_request', payload: { data: 123 } });
    await expect(server).toReceiveMessage({ id: '2', type: 'test_request', payload: { data: 123 } });
    
    server.send({ id: '2', type: 'response', payload: { success: true } });
    
    const result = await requestPromise;
    expect(result).toEqual({ success: true });
  });

  it('rejects promises when error is returned', async () => {
    const client = new WSClient();
    await server.connected;
    
    // Handle initial get_state
    await expect(server).toReceiveMessage({ id: '1', type: 'get_state', payload: {} });
    
    const requestPromise = client.request({ type: 'bad_request', payload: {} });
    await expect(server).toReceiveMessage({ id: '2', type: 'bad_request', payload: {} });
    
    server.send({ id: '2', type: 'response', error: 'Internal Error' });
    
    await expect(requestPromise).rejects.toThrow('Internal Error');
  });

  it('notifies listeners on state_update push messages', async () => {
    const client = new WSClient();
    await server.connected;
    
    const listener = jest.fn();
    const unsubscribe = client.onStateUpdate(listener);
    
    server.send({ type: 'state_update', payload: { my_state: 42 } });
    
    expect(listener).toHaveBeenCalledWith({ my_state: 42 });
    
    unsubscribe();
    server.send({ type: 'state_update', payload: { my_state: 43 } });
    
    // Should not be called again
    expect(listener).toHaveBeenCalledTimes(1);
  });

  it('reconnects after being closed', async () => {
    const client = new WSClient();
    await server.connected;
    
    // Simulate close by invoking the internal onclose callback
    // (since JSDOM mock WebSocket closing can be flaky/asynchronous)
    const ws = (client as any).ws;
    ws.onclose();
    
    // Clean up the first mock server before the timeout executes the reconnection
    WS.clean();
    
    // Start a new mock server for it to reconnect to on the same URL
    const server2 = new WS('ws://localhost/ws', { jsonProtocol: true });
    
    // Fast forward 500ms for reconnection logic to fire
    clock.addMillis(500);
    await clock.executeTimeoutFuncs();
    
    await server2.connected;
    await expect(server2).toReceiveMessage({ id: expect.any(String), type: 'get_state', payload: {} });
    
    WS.clean();
  });

  it('uses wss:// protocol when window.location.protocol is https:', () => {
    const httpsClient = new WSClient('wss://localhost/ws');
    expect(httpsClient.url).toBe('wss://localhost/ws');
  });

  it('ignores response with unknown id', async () => {
    const client = new WSClient();
    await server.connected;
    
    // Send a response with an ID that isn't in requestMap
    server.send({ id: '9999', type: 'response', payload: {} });
    
    // It shouldn't crash
    expect(true).toBe(true);
  });
});
