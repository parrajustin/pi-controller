import { LitElement, html, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import '@material/web/ripple/ripple.js';

export interface Meeting {
  time: string;
  lengthInSeconds: number;
  name: string;
  status: string;
  isActive: boolean;
  meetCode?: string;
}

@customElement('meeting-entry')
export class MeetingEntry extends LitElement {
  @property({ type: Object }) meeting!: Meeting;

  static styles = css`
    .meeting-row {
      display: flex;
      align-items: center;
      padding: 24px 32px;
      border-bottom: 1px solid var(--card-border);
      font-size: 1.5rem;
      position: relative; /* Needed for ripple */
      cursor: pointer;
      -webkit-tap-highlight-color: transparent;
    }
    :host(:last-child) .meeting-row {
      border-bottom: none;
    }
    .meeting-row.active {
      background-color: var(--active-bg);
      color: var(--active-text);
      border-bottom: none;
      box-shadow: 0 4px 8px rgba(0, 0, 0, 0.2);
      border-radius: 16px;
      margin-bottom: 1px;
    }
    .meeting-time {
      width: 140px;
      font-weight: 500;
      color: var(--text-primary);
    }
    .meeting-row.active .meeting-time {
      color: var(--active-text);
    }
    .meeting-name {
      flex: 1;
      font-weight: 400;
      color: var(--text-primary);
    }
    .meeting-row.active .meeting-name {
      color: var(--active-text);
    }
    .meeting-status {
      font-weight: 500;
      color: var(--text-secondary);
    }
    .meeting-row.active .meeting-status {
      color: var(--active-text);
    }
  `;

  private async handleJoinMeeting() {
    if (this.meeting.meetCode) {
      try {
        await fetch('/api/join_meeting', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ code: this.meeting.meetCode }),
        });
      } catch (e) {
        console.error('Failed to join meeting:', e);
      }
    }
  }

  render() {
    return html`
      <div
        class="meeting-row ${this.meeting.isActive ? 'active' : ''}"
        @click=${this.handleJoinMeeting}
      >
        <md-ripple></md-ripple>
        <div class="meeting-time">${this.meeting.time}</div>
        <div class="meeting-name">${this.meeting.name}</div>
        <div class="meeting-status">${this.meeting.status}</div>
      </div>
    `;
  }
}
