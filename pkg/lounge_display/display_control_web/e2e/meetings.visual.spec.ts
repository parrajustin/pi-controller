import { test, expect } from '@playwright/test';

test.describe('Lounge Display Meetings Visual Regression', () => {
  test('should render various meeting list states and interactions correctly', async ({ page }) => {
    test.setTimeout(60000); // Visual tests can take a long time due to snapshotting
    // Set fixed time to 12:00 PM for predictable active/past/future meetings
    const fixedTime = new Date('2026-07-10T12:00:00.000Z');
    await page.clock.install({ time: fixedTime });

    let wsServer: any;
    let messageHandler: ((data: any) => void) | null = null;
    page.on('console', msg => console.log('BROWSER:', msg.text()));

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

    const sendToClient = (msg: any) => {
      if (wsServer) {
        wsServer.send(JSON.stringify(msg));
      }
    };

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

    // Load the page
    const getStatePromise = waitForClientMessage('get_state');
    await page.goto('/', { waitUntil: 'domcontentloaded' });
    const getStateMsg = await getStatePromise;

    // Fast track past setup directly to lounge display
    const calendarEventsPromise1 = waitForClientMessage('calendar_events');
    sendToClient({
      id: getStateMsg.id,
      type: 'response',
      payload: { setup_ready: true, current_node: 'landing', setup_phase: 1000, phase: 'work' }
    });
    
    // Note: lounge-display fetches calendar immediately upon mount
    const calendarEventsMsg = await calendarEventsPromise1;

    // Define standard meetings relative to 12:00 PM
    const standardMeetings = [
      {
        name: 'Past Meeting',
        startTime: '2026-07-10T10:00:00.000Z',
        endTime: '2026-07-10T11:00:00.000Z',
        acceptedStatus: 'accepted',
        description: '',
        meetLink: 'https://meet.google.com/pas-tmee-tin'
      },
      {
        name: 'Current Active Meeting (Blue)',
        startTime: '2026-07-10T11:30:00.000Z',
        endTime: '2026-07-10T12:30:00.000Z', // Active from 11:30 to 12:30, it is 12:00
        acceptedStatus: 'accepted',
        description: '',
        meetLink: 'https://meet.google.com/cur-rent-mee'
      },
      {
        name: 'Future Meeting 1',
        startTime: '2026-07-10T13:00:00.000Z',
        endTime: '2026-07-10T14:00:00.000Z',
        acceptedStatus: 'accepted',
        description: '',
        meetLink: 'https://meet.google.com/fut-urem-ee1'
      },
      {
        name: 'Future Meeting 2',
        startTime: '2026-07-10T15:00:00.000Z',
        endTime: '2026-07-10T16:00:00.000Z',
        acceptedStatus: 'accepted',
        description: '',
        meetLink: 'https://meet.google.com/fut-urem-ee2'
      }
    ];

    sendToClient({
      id: calendarEventsMsg.id,
      type: 'response',
      payload: standardMeetings
    });

    await page.clock.fastForward(1000); // Wait for lit render
    await expect(page).toHaveScreenshot('01-meetings-list-default.png');

    // 1. Test clicking a meeting in its time slot (Current Active Meeting)
    let joinMeetingPromise = waitForClientMessage('join_meeting');
    // We can click by finding the text
    await page.locator('text=Current Active Meeting').click();
    await page.clock.fastForward(500); // Wait for scroll and animation
    await expect(page).toHaveScreenshot('02-click-active-meeting.png');
    console.log('TEST: Screenshot 2 completed!');
    let joinMeetingMsg = await joinMeetingPromise;
    console.log('TEST: joinMeetingPromise resolved!');

    // Reset state by expiring optimistic timeout
    await page.clock.fastForward(30000);

    // 2. Test clicking a meeting NOT in its time slot (Future Meeting 1)
    joinMeetingPromise = waitForClientMessage('join_meeting');
    await page.locator('text=Future Meeting 1').click();
    await page.clock.fastForward(500);
    await expect(page).toHaveScreenshot('03-click-future-meeting.png');
    await joinMeetingPromise;
    
    await page.clock.fastForward(30000);

    // 3. Test clicking a past meeting
    joinMeetingPromise = waitForClientMessage('join_meeting');
    await page.locator('text=Past Meeting').click();
    await page.clock.fastForward(500);
    await expect(page).toHaveScreenshot('04-click-past-meeting.png');
    await joinMeetingPromise;

    await page.clock.fastForward(30000);

    // 4. Test 100 meetings!
    // Trigger calendar reload
    const calendarEventsPromise2 = waitForClientMessage('calendar_events');
    await page.clock.fastForward(60000);
    const calendarEventsMsg2 = await calendarEventsPromise2;

    const massMeetings: any[] = [];
    for (let i = 0; i < 100; i++) {
      massMeetings.push({
        name: `Mass Meeting ${i}`,
        startTime: `2026-07-10T${13 + (Math.floor(i/4) % 11)}:${(i%4)*15 < 10 ? '0' : ''}${(i%4)*15}:00.000Z`,
        endTime: `2026-07-10T${13 + (Math.floor(i/4) % 11)}:${(i%4)*15 + 10 < 10 ? '0' : ''}${(i%4)*15 + 10}:00.000Z`,
        acceptedStatus: 'accepted',
        description: '',
        meetLink: `https://meet.google.com/mas-smee-${i}`
      });
    }

    sendToClient({
      id: calendarEventsMsg2.id,
      type: 'response',
      payload: massMeetings
    });

    await page.clock.fastForward(1000);
    await expect(page).toHaveScreenshot('05-100-meetings.png');

    // Click meeting 50 to see scrolling behavior
    joinMeetingPromise = waitForClientMessage('join_meeting');
    await page.locator('text=Mass Meeting 50').click();
    await page.clock.fastForward(1000); // Give time for smooth scroll
    await expect(page).toHaveScreenshot('06-100-meetings-scrolled-middle.png');
    await joinMeetingPromise;
  });
});
