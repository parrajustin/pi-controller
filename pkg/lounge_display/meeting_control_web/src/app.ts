import { LitElement, html, css } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import { Meeting } from './components/meeting-entry.js';
import './components/meeting-list.js';
import './components/bottom-bar.js';
import splashImg from '../splash.png';
import { parse, isWithinInterval, format, addSeconds, parseISO } from 'date-fns';

interface EventInfo {
  name: string;
  startTime: string;
  endTime: string;
  acceptedStatus: string;
  description: string;
  meetLink: string;
}

@customElement('lounge-display')
export class LoungeDisplay extends LitElement {
  static styles = css`
    :host {
      display: flex;
      flex-direction: column;
      height: 100%;
      width: 100%;
      max-width: 1200px;
      margin: 0 auto;
      box-sizing: border-box;
    }

    /* Header */
    .header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 24px;
      font-size: 1.25rem;
      color: var(--text-secondary);
    }
    .header-left {
      display: flex;
      align-items: center;
      gap: 16px;
      color: var(--text-primary);
    }
    .meet-icon {
      width: 32px;
      height: 32px;
    }
    .meet-icon img {
      width: 100%;
      height: 100%;
      object-fit: contain;
    }
    .header-title {
      font-weight: 500;
    }
    .header-time {
      font-weight: 400;
    }

    /* Ensure meeting list fills the middle */
    meeting-list {
      flex: 1;
      display: flex;
      min-height: 0;
    }

    .no-events-card {
      flex: 1;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 2rem;
      color: var(--text-secondary);
      background-color: rgba(255, 255, 255, 0.05);
      border-radius: 24px;
      margin-bottom: 24px;
      padding: 48px;
      text-align: center;
      box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
    }
    .in-meeting-card {
      flex: 1;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 2.5rem;
      font-weight: 500;
      color: var(--text-primary);
      background-color: var(--card-bg, #ffffff);
      border-radius: 24px;
      margin-bottom: 24px;
      padding: 48px;
      text-align: center;
      box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
    }
  `;

  @state() private serverState = { current_node: '', meeting_code: '' };

  @state() private currentTime = new Date();
  private timer?: ReturnType<typeof setInterval>;

  @state()
  private meetings: Meeting[] = [];

  private fetchTimer?: ReturnType<typeof setInterval>;
  private fetchTimeout?: ReturnType<typeof setTimeout>;

  private async fetchCalendarEvents() {
    try {
      const response = await fetch('/api/calendar_events');
      if (!response.ok) throw new Error('Network error');
      const events: EventInfo[] = await response.json();

      if (!events) {
        this.meetings = [];
        this.updateMeetings();
        return;
      }

      const newMeetings = events.map((e) => {
        const start = parseISO(e.startTime);
        const end = parseISO(e.endTime);
        const lengthInSeconds = (end.getTime() - start.getTime()) / 1000;
        const meetCode = e.meetLink ? e.meetLink.split('/').pop() : undefined;

        return {
          time: format(start, 'h:mm a'),
          lengthInSeconds: Math.max(0, lengthInSeconds),
          name: e.name,
          status: '',
          isActive: false,
          meetCode,
        };
      });
      this.meetings = newMeetings;
      this.updateMeetings();
    } catch (e) {
      console.error('Failed to fetch calendar events', e);
    }
  }

  connectedCallback() {
    super.connectedCallback();
    this.fetchCalendarEvents();

    // Top-of-minute syncing for fetch
    const now = new Date();
    const msUntilNextMinute = (60 - now.getSeconds()) * 1000 - now.getMilliseconds();

    this.fetchTimeout = setTimeout(() => {
      this.fetchCalendarEvents();
      this.fetchTimer = setInterval(() => {
        this.fetchCalendarEvents();
      }, 60000);
    }, msUntilNextMinute);

    this.timer = setInterval(() => {
      this.currentTime = new Date();
      this.updateMeetings();
      this.fetchServerState();
    }, 1000);
  }

  private async fetchServerState() {
    try {
      const response = await fetch('/api/state');
      if (response.ok) {
        this.serverState = await response.json();
      }
    } catch (e) {
      console.error('Failed to fetch server state', e);
    }
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    if (this.timer) {
      clearInterval(this.timer);
    }
    if (this.fetchTimeout) {
      clearTimeout(this.fetchTimeout);
    }
    if (this.fetchTimer) {
      clearInterval(this.fetchTimer);
    }
  }

  private updateMeetings() {
    let changed = false;
    const now = new Date();
    const updated = this.meetings.map((m) => {
      const start = parse(m.time, 'h:mm a', now);
      const end = addSeconds(start, m.lengthInSeconds);
      const isActive = isWithinInterval(this.currentTime, { start, end });
      const status = isActive ? 'Now' : '';
      if (m.isActive !== isActive || m.status !== status) {
        changed = true;
        return { ...m, isActive, status };
      }
      return m;
    });
    if (changed) {
      this.meetings = updated;
    }
  }

  render() {
    const headerTimeStr = `${format(this.currentTime, 'h:mm a')} • ${format(this.currentTime, 'E, MMM d')}`;

    const now = new Date();
    let firstFutureOrActiveIndex = this.meetings.findIndex((m) => {
      const start = parse(m.time, 'h:mm a', now);
      const end = addSeconds(start, m.lengthInSeconds);
      return end > this.currentTime;
    });

    if (firstFutureOrActiveIndex === -1) {
      firstFutureOrActiveIndex = this.meetings.length;
    }

    const startIndex = Math.max(0, firstFutureOrActiveIndex - 1);
    const displayedMeetings = this.meetings.slice(startIndex);

    return html`
      <div class="header">
        <div class="header-left">
          <div class="meet-icon">
            <img src=${splashImg} alt="Lodge Display Logo" />
          </div>
          <div class="header-title">Mountain View De Anza no 194 Lodge Display</div>
        </div>
        <div class="header-time">${headerTimeStr}</div>
      </div>

      ${
        this.serverState.current_node === 'In Meeting' && this.serverState.meeting_code !== 'landing'
          ? html`<div class="in-meeting-card">in meeting ${this.serverState.meeting_code}</div>`
          : displayedMeetings.length > 0
            ? html`<meeting-list .meetings=${displayedMeetings}></meeting-list>`
            : html`<div class="no-events-card">
                No calendar events found! Time to sit back for refreshment and repose!
              </div>`
      }

      <bottom-bar></bottom-bar>
    `;
  }
}
