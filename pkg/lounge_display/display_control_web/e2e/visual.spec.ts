import { test, expect } from '@playwright/test';

test.describe('Lounge Display Visual Regression', () => {
  test('should render all states correctly', async ({ page }) => {
    // Set fixed time so calendar and meeting calculations are deterministic
    const fixedTime = new Date('2026-07-10T12:00:00.000Z');
    await page.clock.install({ time: fixedTime });

    let wsServer: any;
    let messageHandler: ((data: any) => void) | null = null;
    page.on('console', msg => console.log('BROWSER:', msg.text()));

    // Intercept WebSocket
    await page.routeWebSocket(/.*\/ws/, ws => {
      wsServer = ws;
      ws.onMessage(message => {
        console.log('TEST WS MESSAGE:', message.toString());
        const data = JSON.parse(message.toString());
        if (messageHandler) {
          messageHandler(data);
        }
      });
    });

    // A helper to send messages to the client
    const sendToClient = (msg: any) => {
      if (wsServer) {
        wsServer.send(JSON.stringify(msg));
      }
    };

    // A helper to wait for a specific message type from the client
    const waitForClientMessage = (type: string) => {
      return new Promise<any>(resolve => {
        const originalHandler = messageHandler;
        messageHandler = (data) => {
          if (data.type === type) {
            messageHandler = originalHandler;
            resolve(data);
          } else if (originalHandler) {
            originalHandler(data);
          }
        };
      });
    };

    // Setup the wait BEFORE we load the page
    const getStatePromise = waitForClientMessage('get_state');

    // Load the page
    await page.goto('/', { waitUntil: 'domcontentloaded' });

    // Wait for the client to connect and send 'get_state'
    const getStateMsg = await getStatePromise;

    // =========================================
    // STATE 1: Setup - Init Server (Checking Wi-Fi)
    // =========================================
    sendToClient({
      id: getStateMsg.id,
      type: 'response',
      payload: { setup_ready: false, current_node: 'Init Server', setup_phase: 1, phase: 'setup' }
    });

    const hasWifiMsg = await waitForClientMessage('has_wifi');
    sendToClient({ id: hasWifiMsg.id, type: 'response', payload: { internetAccess: true } });

    // Wait for animations
    await page.waitForTimeout(1500); // 1.5s real time, wait, clock is mocked? 
    // Playwright clock mock respects page.waitForTimeout if advance() is not called?
    // Actually, `page.clock.install` pauses all setTimeout unless `page.clock.runFor` or `fastForward` is used!
    // Since our app relies on setTimeout for animations and state updates, we must advance the clock!
    await page.clock.fastForward(1500);

    // Assert Visual: Init Server
    await expect(page).toHaveScreenshot('01-setup-init-server.png');

    // =========================================
    // STATE 2: Setup - Upload Credentials
    // =========================================
    sendToClient({ type: 'state_update', payload: { setup_ready: false, current_node: 'Credentials Phase', setup_phase: 2, phase: 'setup' } });
    
    const getIpMsg = await waitForClientMessage('get_ip');
    sendToClient({ id: getIpMsg.id, type: 'response', payload: { ip: '192.168.1.100' } });

    await page.clock.fastForward(1500);
    await expect(page).toHaveScreenshot('02-setup-upload-credentials.png');

    // Open sidebar in setup
    await page.locator('.toggle-btn').click();
    await page.clock.fastForward(500); // Wait for transition
    await expect(page).toHaveScreenshot('02b-setup-sidebar-open.png');

    // Close sidebar
    await page.locator('.toggle-btn').click();
    await page.clock.fastForward(500);

    // =========================================
    // STATE 3: Setup - Get Auth Token
    // =========================================
    sendToClient({ type: 'state_update', payload: { setup_ready: false, current_node: 'Auth Token Phase', setup_phase: 3, phase: 'setup' } });
    
    const getAuthMsg = await waitForClientMessage('get_auth_url');
    sendToClient({ id: getAuthMsg.id, type: 'response', payload: { url: 'https://accounts.google.com/o/oauth2/auth?test' } });

    await page.clock.fastForward(1500);
    await expect(page).toHaveScreenshot('03-setup-auth-token.png');

    // =========================================
    // STATE 4: Setup - Password Input
    // =========================================
    sendToClient({ type: 'state_update', payload: { setup_ready: false, current_node: 'Password Input Page', setup_phase: 13, phase: 'setup' } });
    
    await page.clock.fastForward(1500);
    await expect(page).toHaveScreenshot('04-setup-password-input.png');

    // Open the on-screen keyboard
    await page.click('#google-password-input');
    await page.clock.fastForward(500); // Wait for keyboard animation
    await expect(page).toHaveScreenshot('05-setup-password-keyboard.png');

    // Close keyboard
    await page.mouse.click(10, 10);
    await page.clock.fastForward(500);

    // The client will display redirecting message
    // Note: When we send setup_ready: true, the UI mounts lounge-display which immediately requests calendar_events!
    const calendarEventsPromise1 = waitForClientMessage('calendar_events');
    sendToClient({ type: 'state_update', payload: { setup_ready: true, setup_phase: 1000, current_node: 'landing' } });
    
    await page.clock.fastForward(1500);
    await expect(page).toHaveScreenshot('06-setup-complete.png');

    // =========================================
    // STATE 6: Lounge Display - Empty
    // =========================================
    // Because setup_ready is true, the app should mount lounge-display
    await page.clock.fastForward(1000); // Let Lit re-render

    const calendarEventsMsg = await calendarEventsPromise1;
    sendToClient({
      id: calendarEventsMsg.id,
      type: 'response',
      payload: [] // Empty calendar
    });

    await page.clock.fastForward(1000);
    await expect(page).toHaveScreenshot('07-lounge-display-empty.png');

    // =========================================
    // STATE 7: Lounge Display - With Meeting
    // =========================================
    // Let's force a refresh by triggering the 60s calendar fetch
    const calendarEventsPromise2 = waitForClientMessage('calendar_events');
    await page.clock.fastForward(60000);
    const calendarEventsMsg2 = await calendarEventsPromise2;
    sendToClient({
      id: calendarEventsMsg2.id,
      type: 'response',
      payload: [
        {
          name: 'Important Team Sync',
          startTime: '2026-07-10T12:30:00.000Z',
          endTime: '2026-07-10T13:30:00.000Z',
          acceptedStatus: 'accepted',
          description: '',
          meetLink: 'https://meet.google.com/abc-defg-hij'
        }
      ]
    });

    await page.clock.fastForward(1000);
    await expect(page).toHaveScreenshot('08-lounge-display-meetings.png');

    // =========================================
    // STATE 8: Lounge Display - Active Meeting
    // =========================================
    // Fast forward to 12:25 (5 minutes before meeting, so it shows 'Starting Soon' or becomes active)
    await page.clock.fastForward(25 * 60 * 1000);
    
    // During this fast forward, the UI polls multiple times. We don't want to hang, 
    // but messageHandler will ignore unexpected messages.
    // Let's just wait a bit to ensure UI updates
    await page.clock.fastForward(1000);
    await expect(page).toHaveScreenshot('09-lounge-display-active-meeting.png');

    // =========================================
    // STATE 9: In Meeting
    // =========================================
    const buttonStatePromise1 = waitForClientMessage('button_state');
    sendToClient({ type: 'state_update', payload: { setup_ready: true, current_node: 'In Meeting', meeting_code: 'abc-defg-hij' } });
    
    await page.clock.fastForward(1000);

    const buttonStateMsg = await buttonStatePromise1;
    
    const buttonStatePromise2 = waitForClientMessage('button_state');
    sendToClient({
      id: buttonStateMsg.id,
      type: 'response',
      payload: { microphone: true, camera: false, hand: false, in_meeting: true }
    });

    await page.clock.fastForward(1000);
    await expect(page).toHaveScreenshot('10-in-meeting-controls.png');
    
    // State 10: Mic off, Hand raised
    const buttonStateMsg2 = await buttonStatePromise2;
    sendToClient({
      id: buttonStateMsg2.id,
      type: 'response',
      payload: { microphone: false, camera: true, hand: true, in_meeting: true }
    });

    await page.clock.fastForward(1000);
    await expect(page).toHaveScreenshot('11-in-meeting-hand-raised.png');

    // Open sidebar in meet control
    await page.locator('.toggle-btn').click();
    await page.clock.fastForward(500); // Wait for transition
    await expect(page).toHaveScreenshot('11b-in-meeting-sidebar-open.png');

    // Click Refresh button to open the restart menu
    await page.getByText('Refresh', { exact: true }).click({ force: true });
    await page.clock.fastForward(500);
    
    // Check that the dialog is open and looks correct
    await expect(page).toHaveScreenshot('12-restart-options-dialog.png');
    
    // Close the restart dialog
    await page.getByText('Refresh', { exact: true }).click({ force: true });
    await page.clock.fastForward(500);

    // Close sidebar
    await page.locator('.toggle-btn').click();
    await page.clock.fastForward(500);
  });
});
