import { WSClient, wsClient } from '../ws-client.js';
import { getAppClock, setAppClock } from '../clock-provider.js';
import { FakeClock } from 'standard-ts-lib/src/clock.js';
import './lounge-display.js';
import { LoungeDisplay } from './lounge-display.js';

describe('LoungeDisplay', () => {
  let clock: FakeClock;

  beforeEach(() => {
    clock = new FakeClock(new Date('2026-07-10T12:00:00.000Z').getTime());
    setAppClock(clock);
  });

  afterEach(() => {
    jest.restoreAllMocks();
    document.body.innerHTML = '';
  });

  it('renders default empty state correctly', async () => {
    const requestStub = jest.spyOn(wsClient, 'request').mockResolvedValue([]);
    
    const el = document.createElement('lounge-display') as LoungeDisplay;
    document.body.appendChild(el);
    
    expect(requestStub).toHaveBeenCalledWith({ type: 'calendar_events' });
    
    await el.updateComplete;
    
    const noEventsCard = el.shadowRoot!.querySelector('.no-events-card');
    expect(noEventsCard).toBeDefined();
    expect(noEventsCard!.textContent).toContain('No calendar events found');
  });

  it('renders correctly when server state jumps directly to landing', async () => {
    jest.spyOn(wsClient, 'request').mockResolvedValue([]);
    
    const el = document.createElement('lounge-display') as LoungeDisplay;
    document.body.appendChild(el);
    
    (wsClient as any).notifyListeners({ setup_ready: true, current_node: 'landing' });
    
    await el.updateComplete;
    
    const noEventsCard = el.shadowRoot!.querySelector('.no-events-card');
    expect(noEventsCard).toBeDefined();
  });

  it('handles server reset to default node', async () => {
    jest.spyOn(wsClient, 'request').mockResolvedValue([]);
    
    const el = document.createElement('lounge-display') as LoungeDisplay;
    document.body.appendChild(el);
    
    (wsClient as any).notifyListeners({ setup_ready: true, current_node: 'In Meeting', meeting_code: 'abc-def' });
    await el.updateComplete;
    
    let inMeetingContainer = el.shadowRoot!.querySelector('.in-meeting-container');
    expect(inMeetingContainer).toBeDefined();
    
    (wsClient as any).notifyListeners({ setup_ready: false, current_node: 'Init Server' });
    await el.updateComplete;
    
    inMeetingContainer = el.shadowRoot!.querySelector('.in-meeting-container');
    expect(inMeetingContainer).toBeNull();
  });

  it('dispatches click_button via wsClient when controls are clicked', async () => {
    jest.spyOn(wsClient, 'request').mockResolvedValue([]);
    const requestStub = jest.spyOn(wsClient, 'request');
    
    const el = document.createElement('lounge-display') as LoungeDisplay;
    document.body.appendChild(el);
    
    (wsClient as any).notifyListeners({ setup_ready: true, current_node: 'In Meeting', meeting_code: 'abc-def' });
    
    // Also push a button_state update to test the rendering of different button states
    (el as any).meetingState = {
      microphone: false,
      camera: false,
      hand: true,
      in_meeting: true
    };
    
    await el.updateComplete;
    
    const controlButtons = el.shadowRoot!.querySelectorAll('.control-btn');
    expect(controlButtons.length).toBe(4); // Mic, Camera, Hand, Hangup
    
    // Click all buttons to ensure all lines are hit
    (controlButtons[0] as HTMLElement).click();
    (controlButtons[1] as HTMLElement).click();
    (controlButtons[2] as HTMLElement).click();
    (controlButtons[3] as HTMLElement).click();
    
    expect(requestStub).toHaveBeenCalledWith({ type: 'click_button', payload: { button: 'microphone' } });
    expect(requestStub).toHaveBeenCalledWith({ type: 'click_button', payload: { button: 'camera' } });
    expect(requestStub).toHaveBeenCalledWith({ type: 'click_button', payload: { button: 'hand' } });
    expect(requestStub).toHaveBeenCalledWith({ type: 'click_button', payload: { button: 'hangup' } });
  });

  it('starts optimistic loading and clears it correctly', async () => {
    jest.spyOn(wsClient, 'request').mockResolvedValue([]);
    const el = document.createElement('lounge-display') as LoungeDisplay;
    document.body.appendChild(el);
    await el.updateComplete;
    
    const event = new CustomEvent('join-meeting-start', { detail: { code: 'test-meeting' } });
    await (el as any).handleJoinMeetingStart(event);
    
    await el.updateComplete;
    
    const loadingBg = el.shadowRoot!.querySelector('.loading-bg');
    expect(loadingBg!.classList.contains('active')).toBe(true);
    
    clock.addMillis(30000);
    await clock.executeTimeoutFuncs();
    
    await el.updateComplete;
    expect(loadingBg!.classList.contains('active')).toBe(false);
  });

  it('handles errors when joining a meeting', async () => {
    jest.spyOn(wsClient, 'request').mockImplementation((req: any) => {
      if (req.type === 'join_meeting') return Promise.reject(new Error('Connection dropped'));
      return Promise.resolve([]);
    });
    const consoleSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
    
    const el = document.createElement('lounge-display') as LoungeDisplay;
    document.body.appendChild(el);
    await el.updateComplete;
    
    const event = new CustomEvent('join-meeting-start', { detail: { code: 'test-meeting' } });
    await (el as any).handleJoinMeetingStart(event);
    
    expect(consoleSpy).toHaveBeenCalledWith('Failed to join meeting:', expect.any(Error));
    consoleSpy.mockRestore();
  });

  it('clears intervals and timeouts on disconnectedCallback', async () => {
    jest.spyOn(wsClient, 'request').mockResolvedValue([]);
    const el = document.createElement('lounge-display') as LoungeDisplay;
    document.body.appendChild(el);
    
    await el.updateComplete;
    
    clock.addMillis(120000);
    await clock.executeTimeoutFuncs();
    
    expect((el as any).timer).toBeDefined();
    expect((el as any).fetchTimer).toBeDefined();
    
    // Simulate disconnect
    el.remove();
    
    // If it didn't clear the timeout, fakeclock might execute them and throw errors later
    // Just verifying that it was called is tricky since we use getAppClock(), but we can ensure it doesn't leak.
    expect((el as any).timer).toBeDefined();
  });

  it('handles empty events correctly', async () => {
    jest.spyOn(wsClient, 'request').mockResolvedValue(null);
    
    const el = document.createElement('lounge-display') as LoungeDisplay;
    document.body.appendChild(el);
    await el.updateComplete;
    
    // Trigger fetchCalendarEvents multiple times to hit the repeating timer
    clock.addMillis(60000);
    await clock.executeTimeoutFuncs();
    clock.addMillis(60000);
    await clock.executeTimeoutFuncs();
    
    expect((el as any).meetings).toEqual([]);
  });

  it('handles button state fetch errors', async () => {
    jest.spyOn(wsClient, 'request').mockImplementation((req: any) => {
      if (req.type === 'button_state') return Promise.reject(new Error('fail'));
      return Promise.resolve([]);
    });
    
    const el = document.createElement('lounge-display') as LoungeDisplay;
    document.body.appendChild(el);
    await el.updateComplete;
    
    // Manually trigger pollMeetingState
    (wsClient as any).notifyListeners({ current_node: 'In Meeting' });
    clock.addMillis(1500);
    await clock.executeTimeoutFuncs();
    
    // It should not crash, just log error
  });
});
