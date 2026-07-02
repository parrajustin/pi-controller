import { LitElement, html, css } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import { Meeting } from './components/meeting-entry.js';
import './components/meeting-list.js';
import './components/bottom-bar.js';
import splashImg from '../splash.png';

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

  @state()
  private meetings: Meeting[] = [
    { time: '10:00 AM', name: 'Team Meeting', status: 'Now', isActive: true },
    { time: '1:00 PM', name: 'Mobile Team Retro', status: '', isActive: false },
    { time: '2:00 PM', name: 'Launch party!', status: '', isActive: false },
    { time: '4:00 PM', name: 'Illustration Review', status: '', isActive: false },
  ];

  render() {
    return html`
      <div class="header">
        <div class="header-left">
          <div class="meet-icon">
            <img src=${splashImg} alt="Lodge Display Logo" />
          </div>
          <div class="header-title">Mountain View De Anza no 194 Lodge Display</div>
        </div>
        <div class="header-time">10:00 AM • Mon, July 7</div>
      </div>

      <meeting-list .meetings=${this.meetings}></meeting-list>

      <bottom-bar></bottom-bar>
    `;
  }
}
