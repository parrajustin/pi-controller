import { LitElement, html, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { Meeting } from './meeting-entry.js';
import './meeting-entry.js';

@customElement('meeting-list')
export class MeetingList extends LitElement {
  @property({ type: Array }) meetings: Meeting[] = [];
  @property({ type: Boolean, reflect: true }) isLoading = false;
  @property({ type: String }) selectedCode = '';

  static styles = css`
    :host {
      display: flex;
      width: 100%;
      height: 100%;
    }
    .main-content {
      flex: 1;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: flex-start;
      width: 100%;
      min-height: 0;
    }
    .meetings-container {
      width: 100%;
      max-width: 800px;
      background-color: var(--card-bg);
      border-radius: 16px;
      overflow-y: auto;
      flex-shrink: 1;
    }
    :host([isloading]) .meetings-container {
      overflow: visible;
      background-color: transparent;
    }
    .spacer {
      transition: flex-grow 0.5s ease;
      flex-grow: 0;
    }
    :host([isloading]) .spacer {
      flex-grow: 1;
    }
  `;

  render() {
    return html`
      <div class="main-content">
        <div class="spacer"></div>
        <div class="meetings-container">
          ${this.meetings.map((meeting) => html`<meeting-entry .meeting=${meeting} ?isLoading=${this.isLoading} ?isSelected=${this.selectedCode === meeting.meetCode}></meeting-entry>`)}
        </div>
        <div class="spacer"></div>
      </div>
    `;
  }
}
