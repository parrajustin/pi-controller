import { LitElement, html, css } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import '@material/web/progress/linear-progress.js';
import '@material/web/checkbox/checkbox.js';
import splashImg from '../splash.png';

@customElement('setup-display')
export class SetupDisplay extends LitElement {
  static styles = css`
    :host {
      display: flex;
      width: 100%;
      height: 100%;
      /* 800x400 display target */
      max-width: 800px;
      max-height: 400px;
      margin: 0 auto;
      background-color: var(--bg-color, #202124);
      color: var(--text-primary, #e8eaed);
      overflow: hidden;
    }

    .main-layout {
      display: flex;
      width: 100%;
      height: 100%;
    }

    .splash-section {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      width: 100%;
      height: 100%;
      /* M3 Emphasized Decelerate */
      transition: width 500ms cubic-bezier(0.2, 0, 0, 1);
      padding: 20px;
      box-sizing: border-box;
    }

    .splash-section.split {
      width: 50%;
    }

    .splash-img {
      width: 100%;
      max-width: 400px;
      object-fit: contain;
      transition: max-width 500ms cubic-bezier(0.2, 0, 0, 1);
    }

    .splash-section.split .splash-img {
      max-width: 250px;
    }

    .progress-container {
      width: 100%;
      max-width: 400px;
      display: flex;
      flex-direction: column;
      gap: 16px;
      align-items: center;
      margin-top: 40px;
      transition: all 500ms cubic-bezier(0.2, 0, 0, 1);
    }
    
    .splash-section.split .progress-container {
      max-width: 250px;
    }

    .loading-text {
      font-size: 1.2rem;
      font-weight: 500;
      color: var(--text-secondary, #9aa0a6);
      text-align: center;
    }

    md-linear-progress {
      width: 100%;
      --md-linear-progress-active-indicator-color: #a8c7fa;
      --md-linear-progress-track-color: #3c4043;
    }

    .checklist-section {
      width: 0%;
      opacity: 0;
      display: flex;
      flex-direction: column;
      justify-content: center;
      padding: 0;
      overflow: hidden;
      /* M3 Emphasized Decelerate */
      transition: all 500ms cubic-bezier(0.2, 0, 0, 1);
      box-sizing: border-box;
      background-color: var(--card-bg, #28292c);
    }

    .checklist-section.show {
      width: 50%;
      opacity: 1;
      padding: 40px;
    }

    .checklist-title {
      font-size: 1.5rem;
      font-weight: 500;
      margin-bottom: 32px;
      color: var(--text-primary, #e8eaed);
      white-space: nowrap;
    }

    .check-item {
      display: flex;
      align-items: center;
      gap: 16px;
      margin-bottom: 24px;
      font-size: 1.1rem;
      color: var(--text-primary, #e8eaed);
      white-space: nowrap;
      cursor: default;
    }

    md-checkbox {
      --md-checkbox-checked-container-color: #a8c7fa;
      --md-checkbox-checked-icon-color: #062e6f;
    }
  `;

  @state() private allClear = false;
  @state() private showChecklist = false;

  connectedCallback() {
    super.connectedCallback();
    this.pollStatus();

    // Trigger the M3 transition to reveal the checklist shortly after load
    setTimeout(() => {
      this.showChecklist = true;
    }, 1200);
  }

  private async pollStatus() {
    const check = async () => {
      if (this.allClear) return;
      try {
        const res = await fetch('/api/status');
        if (res.ok) {
          const data = await res.json();
          if (data.status === 'ready') {
            this.allClear = true;
            setTimeout(() => {
              window.location.href = '/';
            }, 1500);
            return;
          }
        }
      } catch (e) {
        // Ignore fetch errors, keep trying
      }
      setTimeout(check, 2000);
    };
    check();
  }

  render() {
    if (this.allClear) {
      return html`
        <div class="main-layout">
          <div class="splash-section">
            <img class="splash-img" src=${splashImg} alt="Lodge Display Splash" />
            <div class="progress-container">
              <div class="loading-text">Setup Complete! Redirecting...</div>
            </div>
          </div>
        </div>
      `;
    }

    return html`
      <div class="main-layout">
        <div class="splash-section ${this.showChecklist ? 'split' : ''}">
          <img class="splash-img" src=${splashImg} alt="Lodge Display Splash" />
          
          <div class="progress-container">
            <div class="loading-text">Waiting for all clear signal...</div>
            <md-linear-progress indeterminate></md-linear-progress>
          </div>
        </div>

        <div class="checklist-section ${this.showChecklist ? 'show' : ''}">
          <div class="checklist-title">Setup Progress</div>
          
          <label class="check-item">
            <md-checkbox checked></md-checkbox>
            <span>Connect to Wifi</span>
          </label>
          
          <label class="check-item">
            <md-checkbox></md-checkbox>
            <span>Upload credentials.json</span>
          </label>
          
          <label class="check-item">
            <md-checkbox></md-checkbox>
            <span>Link auth token</span>
          </label>
        </div>
      </div>
    `;
  }
}
