import { LitElement, html, css } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import { Meeting } from './components/meeting-entry.js';
import './components/meeting-list.js';
import './components/bottom-bar.js';
import splashImg from '../splash.png';
import { parse, isWithinInterval, format, addSeconds } from 'date-fns';

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
  `;

  @state() private currentTime = new Date();
  private timer?: ReturnType<typeof setInterval>;

  @state()
  private meetings: Meeting[] = [
    { time: '10:30 AM', lengthInSeconds: 1800, name: 'Design Sync', status: '', isActive: false },
    { time: '11:30 AM', lengthInSeconds: 1800, name: 'Mobile Team Retro', status: '', isActive: false },
    { time: '2:00 PM', lengthInSeconds: 3600, name: 'Launch party!', status: '', isActive: false },
    { time: '4:00 PM', lengthInSeconds: 3600, name: 'Illustration Review', status: '', isActive: false },
  ];

  connectedCallback() {
    super.connectedCallback();
    this.updateMeetings();
    this.timer = setInterval(() => {
      this.currentTime = new Date();
      this.updateMeetings();
    }, 1000);
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    if (this.timer) {
      clearInterval(this.timer);
    }
  }

  private updateMeetings() {
    let changed = false;
    const now = new Date();
    const updated = this.meetings.map(m => {
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
    let firstFutureOrActiveIndex = this.meetings.findIndex(m => {
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

      <meeting-list .meetings=${displayedMeetings}></meeting-list>

      <bottom-bar></bottom-bar>
    `;
  }
}
