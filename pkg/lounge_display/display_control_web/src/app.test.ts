import WS from 'jest-websocket-mock';
import { FakeClock } from 'standard-ts-lib/src/clock.js';
import { setAppClock } from './clock-provider.js';


let clock: FakeClock;
let server: WS;

// Wait for Lit to finish rendering
async function nextFrame() {
  return new Promise(resolve => requestAnimationFrame(resolve));
}

// Since FakeClock timeouts require executeTimeoutFuncs to be called,
// we create a helper to jump time and execute timeouts
async function advanceTime(ms: number) {
  clock.addMillis(ms);
  await clock.executeTimeoutFuncs();
  await nextFrame();
}

describe('Display Control App Integration', () => {
  beforeEach(() => {
    // Start at a fixed time (e.g., 2026-07-10T12:00:00.000Z)
    clock = new FakeClock(new Date('2026-07-10T12:00:00.000Z').getTime());
    setAppClock(clock);

    // Jest JSDOM sets window.location.host to 'localhost'
    server = new WS('ws://localhost/ws', { jsonProtocol: true });

    // Clean up DOM
    document.body.innerHTML = '';
  });

  afterEach(() => {
    WS.clean();
  });

  it('navigates from setup to landing page to meeting and back', async () => {
    // Dynamically import so ws-client connects AFTER the mock server is running
    await import('./app.js');

    const app = document.createElement('display-control-app');
    document.body.appendChild(app);

    await nextFrame();
    
    // Initial snapshot - Should show setup display initializing
    expect(document.body.innerHTML).toMatchSnapshot('1 - Initial Setup View');

    // Wait for the client to connect
    await server.connected;
    
    // The client immediately requests `get_state`. Let's respond.
    await expect(server).toReceiveMessage({ id: '1', type: 'get_state', payload: {} });
    server.send({ id: '1', type: 'response', payload: { setup_ready: false, current_node: 'Init Server', setup_phase: 1, phase: 'setup' } });
    
    await nextFrame();
    
    // Simulate some time passing to trigger UI transitions in setup-display
    await advanceTime(1500);
    await expect(server).toReceiveMessage({ id: '2', type: 'has_wifi', payload: {} });
    server.send({ id: '2', type: 'response', payload: { internetAccess: true } });

    await nextFrame();
    
    // Simulate some time passing to trigger UI transitions in setup-display
    await advanceTime(1500);
    expect(document.body.innerHTML).toMatchSnapshot('2 - Setup Init Server View');

    // Update state to setup complete and go to landing
    server.send({ type: 'state_update', payload: { setup_ready: true, current_node: 'landing' } });
    await nextFrame();
    
    // It should now transition to lounge-display
    expect(document.body.innerHTML).toMatchSnapshot('3 - Lounge Display Empty');

    // Now lounge-display will fetch calendar events because it's mounted
    // Wait for the request
    await expect(server).toReceiveMessage({ id: '3', type: 'calendar_events', payload: {} });
    server.send({ id: '3', type: 'response', payload: [
      {
        name: 'Test Meeting',
        startTime: '2026-07-10T12:00:05.000Z',
        endTime: '2026-07-10T13:00:00.000Z',
        acceptedStatus: 'accepted',
        description: '',
        meetLink: 'https://meet.google.com/abc-defg-hij'
      }
    ]});

    await nextFrame();
    // After receiving events, it should show the meeting in the list
    expect(document.body.innerHTML).toMatchSnapshot('4 - Lounge Display with Meetings');

    // Let's advance time to 12:00:05 so the meeting becomes active
    await advanceTime(5000);
    expect(document.body.innerHTML).toMatchSnapshot('5 - Lounge Display Meeting Active');

    // Now let's simulate joining the meeting via state update from server
    server.send({ type: 'state_update', payload: { setup_ready: true, current_node: 'In Meeting', meeting_code: 'abc-defg-hij' } });
    await nextFrame();
    // Since we entered a meeting, lounge-display should poll for button_state
    await advanceTime(1000);
    await expect(server).toReceiveMessage({ id: '4', type: 'button_state', payload: {} });
    server.send({ id: '4', type: 'response', payload: { microphone: true, camera: false, hand: false, in_meeting: true } });
    
    await nextFrame();
    expect(document.body.innerHTML).toMatchSnapshot('6 - In Meeting View');

    // Now let's leave the meeting
    server.send({ type: 'state_update', payload: { setup_ready: true, current_node: 'landing' } });
    await nextFrame();
    
    expect(document.body.innerHTML).toMatchSnapshot('7 - Back to Landing Page');
  });

  it('handles fetch error gracefully', async () => {
    (global.fetch as jest.Mock).mockRejectedValueOnce(new Error('Network error'));
    
    const consoleSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
    
    // We already imported app.js in the first test, so the component is registered.
    const app = document.createElement('display-control-app');
    document.body.appendChild(app);

    await nextFrame();
    
    expect(consoleSpy).toHaveBeenCalledWith('Failed to fetch setup status', expect.any(Error));
    consoleSpy.mockRestore();
  });
});
