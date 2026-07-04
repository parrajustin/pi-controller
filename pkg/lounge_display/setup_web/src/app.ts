import { LitElement, html, css, TemplateResult } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import '@material/web/progress/linear-progress.js';
import '@material/web/checkbox/checkbox.js';
import '@material/web/textfield/outlined-text-field.js';
import '@material/web/button/filled-button.js';
import splashImg from '../splash.png';
import { WrapPromise } from 'standard-ts-lib/src/wrap_promise.js';
import QRCode from 'qrcode';

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
      padding: 20px;
    }

    .checklist-title {
      font-size: 1.5rem;
      font-weight: 500;
      margin-bottom: 16px;
      color: var(--text-primary, #e8eaed);
      white-space: nowrap;
      flex-shrink: 0;
    }

    .stage-container {
      display: flex;
      flex-direction: column;
      overflow: hidden;
      opacity: 1;
      max-height: 400px;
      transition: all 500ms cubic-bezier(0.2, 0, 0, 1);
      flex-shrink: 0;
    }
    
    .stage-container.completed-hide {
      opacity: 0;
      max-height: 0;
      margin: 0;
      padding: 0;
      border: 0;
    }

    .check-item {
      display: flex;
      align-items: center;
      gap: 16px;
      margin-bottom: 12px;
      font-size: 1.1rem;
      color: var(--text-primary, #e8eaed);
      white-space: nowrap;
      cursor: default;
    }

    md-checkbox {
      --md-checkbox-checked-container-color: #a8c7fa;
      --md-checkbox-checked-icon-color: #062e6f;
    }

    .extra-html-container {
      margin-left: 36px;
      margin-bottom: 0px;
      font-size: 0.95rem;
      color: var(--text-secondary, #9aa0a6);
      overflow: hidden;
      display: flex;
      flex-direction: column;
      /* M3 animations for sliding open */
      max-height: 0;
      opacity: 0;
      transition: max-height 500ms cubic-bezier(0.2, 0, 0, 1),
                  opacity 500ms cubic-bezier(0.2, 0, 0, 1),
                  margin-bottom 500ms cubic-bezier(0.2, 0, 0, 1),
                  flex-grow 500ms cubic-bezier(0.2, 0, 0, 1);
    }

    .extra-html-container.open {
      max-height: 400px; /* arbitrary max height for smooth expansion */
      opacity: 1;
      margin-bottom: 16px;
      flex-grow: 1;
      min-height: 0;
    }

    pre {
      background: #171717;
      padding: 12px;
      border-radius: 8px;
      overflow-x: auto;
      margin-top: 8px;
      color: #a8c7fa;
      font-family: monospace;
    }
  `;

  @state() private showChecklist = false;
  
  @state() private stage = 1;
  @state() private isLoading = true;
  @state() private statusPageText = 'Checking if there is internet';
  @state() private extraHtml: TemplateResult | undefined = undefined;
  
  @state() private allClear = false;
  @state() private countdown = 15;

  connectedCallback() {
    super.connectedCallback();
    
    // Trigger the M3 transition to reveal the checklist shortly after load
    setTimeout(() => {
      this.showChecklist = true;
      this.startPolling();
    }, 1200);
  }

  private async handleTokenSubmit() {
    if (!this.shadowRoot) return;
    const input = this.shadowRoot.querySelector('#token-input') as HTMLInputElement;
    if (!input || !input.value) return;

    let code = input.value;
    try {
      const url = new URL(input.value);
      const codeParam = url.searchParams.get('code');
      if (codeParam) {
        code = codeParam;
      }
    } catch (e) {
      // Not a URL, treat as raw code
    }

    const res = await WrapPromise(fetch('/api/token', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ code })
    }), 'Failed to post token');

    if (res.ok && res.safeUnwrap().ok) {
      input.value = '';
      this.statusPageText = 'Token submitted, verifying...';
    } else {
      alert('Failed to submit token');
    }
  }

  // --- State Machine Polling ---

  private async startPolling() {
    while (!this.allClear) {
      if (this.stage === 1) {
        const passed = await this.checkStage1();
        if (passed) {
          await new Promise(r => setTimeout(r, 3000));
          this.stage = 2;
          this.isLoading = true;
          this.statusPageText = 'Checking for credentials';
          this.extraHtml = undefined;
          continue;
        } else {
          this.isLoading = false;
          this.statusPageText = 'Please Connect this Pi to the internet';
          this.extraHtml = html`<pre><code>mvda-lounge-display-wifi-portal</code></pre>`;
        }
      } else if (this.stage === 2) {
        const passed = await this.checkStage2();
        if (passed) {
          await new Promise(r => setTimeout(r, 3000));
          this.stage = 3;
          this.isLoading = true;
          this.statusPageText = 'Checking for auth token';
          this.extraHtml = undefined;
          continue;
        } else {
          this.isLoading = false;
          this.statusPageText = 'Upload Credentials to display';
          
          let hostIp = window.location.host;
          const ipRes = await WrapPromise(fetch('/api/ip'), 'Failed to fetch IP');
          if (ipRes.ok && ipRes.safeUnwrap().ok) {
            const ipData = await WrapPromise(ipRes.safeUnwrap().json(), 'Failed to parse IP json');
            if (ipData.ok && ipData.safeUnwrap().ip) {
              const port = window.location.port ? `:${window.location.port}` : '';
              hostIp = `${ipData.safeUnwrap().ip}${port}`;
            }
          }
          
          const uploadUrl = `http://${hostIp}/upload.html`;
          
          const qrRes = await WrapPromise(QRCode.toDataURL(uploadUrl, {
            margin: 2,
            width: 180,
            color: {
              dark: '#000000',
              light: '#ffffff'
            }
          }), 'failed to generate qr');

          if (qrRes.ok) {
            this.extraHtml = html`
              <div style="display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; margin-top: 8px; flex-grow: 1; min-height: 0;">
                <img src="${qrRes.safeUnwrap()}" alt="QR Code to Upload Page" style="border-radius: 8px; max-height: 100%; min-height: 0; object-fit: contain;" />
                <a href="${uploadUrl}" style="color: #a8c7fa; text-decoration: none; word-break: break-all; text-align: center; font-size: 1rem; flex-shrink: 0;">${uploadUrl}</a>
              </div>
            `;
          } else {
            this.extraHtml = html`
              <div style="display: flex; justify-content: center; margin-top: 16px;">
                <a href="${uploadUrl}" style="color: #a8c7fa; text-decoration: none;">${uploadUrl}</a>
              </div>
            `;
          }
        }
      } else if (this.stage === 3) {
        const hasAuth = await this.checkStage3();
        if (hasAuth) {
          await new Promise(r => setTimeout(r, 3000));
          this.stage = 4;
          this.isLoading = true;
          this.statusPageText = 'Checking if we can fetch calendar events';
          this.extraHtml = undefined;
          continue;
        } else {
          this.isLoading = false;
          this.statusPageText = 'Authorize the display';
          
          const authUrlRes = await WrapPromise(fetch('/api/auth_url'), 'Failed to fetch auth url');
          if (authUrlRes.ok && authUrlRes.safeUnwrap().ok) {
            const authUrlData = await WrapPromise(authUrlRes.safeUnwrap().json(), 'Failed to parse auth url json');
            if (authUrlData.ok && authUrlData.safeUnwrap().url) {
              const url = authUrlData.safeUnwrap().url;
              if (url) {
                if (!this.extraHtml) {
                  const qrRes = await WrapPromise(QRCode.toDataURL(url, {
                    margin: 2,
                    width: 180,
                    color: {
                      dark: '#000000',
                      light: '#ffffff'
                    }
                  }), 'failed to generate qr');
        
                  if (qrRes.ok) {
                    this.extraHtml = html`
                      <div style="display: flex; flex-direction: column; align-items: stretch; justify-content: flex-start; gap: 8px; margin-top: 8px; flex-grow: 1; min-height: 0; overflow-y: auto;">
                        <div style="display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 4px;">
                          <img src="${qrRes.safeUnwrap()}" alt="QR Code for Auth URL" style="border-radius: 8px; max-height: 150px; min-height: 0; object-fit: contain;" />
                          <a href="${url}" target="_blank" style="color: #a8c7fa; text-decoration: none; word-break: break-all; text-align: center; font-size: 0.9rem; flex-shrink: 0;">Open Auth URL</a>
                        </div>
                        <div style="font-size: 0.85rem; color: var(--text-secondary, #9aa0a6); margin-top: 4px; text-wrap: wrap;">
                          Google may redirect to a broken 'localhost' page. Copy that full URL and paste it below:
                        </div>
                        <div style="display: flex; gap: 8px; align-items: center; margin-bottom: 8px;">
                          <md-outlined-text-field id="token-input" label="Paste URL or Code" style="flex-grow: 1;"></md-outlined-text-field>
                          <md-filled-button @click=${this.handleTokenSubmit}>Submit</md-filled-button>
                        </div>
                      </div>
                    `;
                  }
                }
              } else {
                this.isLoading = true;
                this.statusPageText = 'Generating Auth URL...';
                this.extraHtml = undefined;
              }
            }
          }
        }
      } else if (this.stage === 4) {
        const hasCalendar = await this.checkStage4();
        if (hasCalendar) {
          await new Promise(r => setTimeout(r, 3000));
          this.stage = 5;
          this.isLoading = true;
          this.statusPageText = 'Finalization';
          this.extraHtml = undefined;
          continue;
        } else {
          this.isLoading = true;
          this.statusPageText = 'Checking if we can fetch calendar events';
          this.extraHtml = undefined;
        }
      } else if (this.stage === 5) {
        const isFinalized = await this.checkStage5();
        if (isFinalized) {
          await new Promise(r => setTimeout(r, 3000));
          this.stage = 6;
          this.isLoading = false;
          this.statusPageText = 'Setup was successfull refreshing in 15';
          this.extraHtml = undefined;
          this.startCountdown();
          return; // exit the polling loop completely since countdown takes over
        } else {
          this.isLoading = true;
          this.statusPageText = 'Finalization';
          this.extraHtml = undefined;
        }
      }
      
      await new Promise(resolve => setTimeout(resolve, 15000));
    }
  }

  private async checkStage1(): Promise<boolean> {
    const res = await WrapPromise(fetch('/api/has_wifi'), 'failed fetch');
    if (!res.ok || !res.safeUnwrap().ok) return false;
    
    const dataRes = await WrapPromise(res.safeUnwrap().json(), 'failed json');
    if (!dataRes.ok || !dataRes.safeUnwrap().internetAccess) return false;
    
    return true;
  }

  private async checkStage2(): Promise<boolean> {
    const res = await WrapPromise(fetch('/api/has_cred'), 'failed fetch');
    if (!res.ok || !res.safeUnwrap().ok) return false;
    
    const dataRes = await WrapPromise(res.safeUnwrap().json(), 'failed json');
    if (!dataRes.ok || !dataRes.safeUnwrap().hasCreds) return false;
    
    return true;
  }

  private async checkStage3(): Promise<boolean> {
    const res = await WrapPromise(fetch('/auth/has_token'), 'failed fetch');
    if (!res.ok || !res.safeUnwrap().ok) return false;
    
    const dataRes = await WrapPromise(res.safeUnwrap().json(), 'failed json');
    if (!dataRes.ok || !dataRes.safeUnwrap().hasToken) return false;
    
    return true;
  }

  private async checkStage4(): Promise<boolean> {
    const res = await WrapPromise(fetch('/auth/has_calendar'), 'failed fetch');
    if (!res.ok || !res.safeUnwrap().ok) return false;
    
    const dataRes = await WrapPromise(res.safeUnwrap().json(), 'failed json');
    if (!dataRes.ok || !dataRes.safeUnwrap().hasCalendar) return false;
    
    return true;
  }

  private async checkStage5(): Promise<boolean> {
    const res = await WrapPromise(fetch('/auth/finalize'), 'failed fetch');
    if (!res.ok || !res.safeUnwrap().ok) return false;
    
    const dataRes = await WrapPromise(res.safeUnwrap().json(), 'failed json');
    if (!dataRes.ok || !dataRes.safeUnwrap().success) return false;
    
    return true;
  }

  private startCountdown() {
    this.countdown = 15;
    const interval = setInterval(() => {
      this.countdown--;
      this.statusPageText = `Setup was successfull refreshing in ${this.countdown}`;
      if (this.countdown <= 0) {
        clearInterval(interval);
        window.location.reload();
      }
    }, 1000);
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
            <div class="loading-text">${this.statusPageText}</div>
            ${this.isLoading ? html`<md-linear-progress indeterminate></md-linear-progress>` : ''}
          </div>
        </div>

        <div class="checklist-section ${this.showChecklist ? 'show' : ''}">
          <div class="checklist-title">Setup Progress</div>
          
          <div class="stage-container ${this.stage > 1 ? 'completed-hide' : ''}">
            <label class="check-item">
              <md-checkbox ?checked=${this.stage > 1} disabled></md-checkbox>
              <span>Connect to Wifi</span>
            </label>
            <div class="extra-html-container ${this.stage === 1 && this.extraHtml ? 'open' : ''}">
              ${this.stage === 1 ? this.extraHtml : ''}
            </div>
          </div>
          
          <div class="stage-container ${this.stage > 2 ? 'completed-hide' : ''}">
            <label class="check-item">
              <md-checkbox ?checked=${this.stage > 2} disabled></md-checkbox>
              <span>Upload credentials.json</span>
            </label>
            <div class="extra-html-container ${this.stage === 2 && this.extraHtml ? 'open' : ''}">
              ${this.stage === 2 ? this.extraHtml : ''}
            </div>
          </div>
          
          <div class="stage-container ${this.stage > 3 ? 'completed-hide' : ''}">
            <label class="check-item">
              <md-checkbox ?checked=${this.stage > 3} disabled></md-checkbox>
              <span>Get auth token</span>
            </label>
            <div class="extra-html-container ${this.stage === 3 && this.extraHtml ? 'open' : ''}">
              ${this.stage === 3 ? this.extraHtml : ''}
            </div>
          </div>
          
          <div class="stage-container ${this.stage > 4 ? 'completed-hide' : ''}">
            <label class="check-item">
              <md-checkbox ?checked=${this.stage > 4} disabled></md-checkbox>
              <span>Got Calendar Events</span>
            </label>
            <div class="extra-html-container ${this.stage === 4 && this.extraHtml ? 'open' : ''}">
              ${this.stage === 4 ? this.extraHtml : ''}
            </div>
          </div>

          <div class="stage-container ${this.stage > 5 ? 'completed-hide' : ''}">
            <label class="check-item">
              <md-checkbox ?checked=${this.stage > 5} disabled></md-checkbox>
              <span>Finalized</span>
            </label>
            <div class="extra-html-container ${this.stage === 5 && this.extraHtml ? 'open' : ''}">
              ${this.stage === 5 ? this.extraHtml : ''}
            </div>
          </div>

          ${this.stage === 6 ? html`
            <div class="check-item" style="justify-content: center; font-size: 1.3rem; margin-top: 20px; text-align: center; text-wrap: wrap;">
              ${this.statusPageText}
            </div>
          ` : ''}
        </div>
      </div>
    `;
  }
}
