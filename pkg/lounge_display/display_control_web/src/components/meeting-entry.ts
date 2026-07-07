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
  @property({ type: Boolean, reflect: true }) isLoading = false;
  @property({ type: Boolean, reflect: true }) isSelected = false;

  static styles = css`
    :host {
      display: block;
      transition: all 0.5s cubic-bezier(0.4, 0.0, 0.2, 1);
      max-height: 150px;
      opacity: 1;
      overflow: hidden;
      margin: 0;
    }
    :host([isloading]:not([isselected])) {
      max-height: 0;
      opacity: 0;
      pointer-events: none;
      margin: 0;
    }
    :host([isloading]) .meeting-row {
      pointer-events: none;
    }
    :host([isselected]) {
      overflow: visible;
      z-index: 10;
    }
    :host([isselected]) .meeting-row {
      border: 2px solid var(--active-text);
      box-shadow: 0 0 0 0 rgba(168, 199, 250, 0.7);
      animation: pulse 1.5s infinite;
      border-radius: 16px;
    }
    @keyframes pulse {
      0% {
        box-shadow: 0 0 0 0 rgba(168, 199, 250, 0.8),
                    0 0 0 0 rgba(168, 199, 250, 0.8);
      }
      50% {
        box-shadow: 0 0 0 40px rgba(168, 199, 250, 0.4),
                    0 0 0 0 rgba(168, 199, 250, 0.8);
      }
      100% {
        box-shadow: 0 0 0 80px rgba(168, 199, 250, 0),
                    0 0 0 40px rgba(168, 199, 250, 0);
      }
    }
    .meeting-row {
      display: flex;
      align-items: center;
      padding: 24px 32px;
      border-bottom: 1px solid var(--card-border);
      font-size: 1.5rem;
      position: relative; /* Needed for ripple */
      cursor: pointer;
      -webkit-tap-highlight-color: transparent;
      transition: all 0.3s ease;
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
    if (this.isLoading) return; // Prevent clicks while transitioning

    if (this.meeting.meetCode) {
      this.dispatchEvent(
        new CustomEvent('join-meeting-start', {
          detail: { code: this.meeting.meetCode },
          bubbles: true,
          composed: true,
        })
      );
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
