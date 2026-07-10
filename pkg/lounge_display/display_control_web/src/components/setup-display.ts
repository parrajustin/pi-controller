import { LitElement, html, css, TemplateResult } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import '@material/web/progress/linear-progress.js';
import '@material/web/checkbox/checkbox.js';
import '@material/web/textfield/outlined-text-field.js';
import '@material/web/button/filled-button.js';
import splashImg from '../../splash.png';
import { WrapPromise } from 'standard-ts-lib/src/wrap_promise.js';
import QRCode from 'qrcode';
import './virtual-keyboard.js';
import { KeyPressedEvent } from './virtual-keyboard.js';
import { wsClient } from '../ws-client.js';
import { getAppClock } from '../clock-provider.js';

const SETUP_STEPS = [
  { phase: 1, text: 'Init Server' },
  { phase: 2, text: 'Upload credentials.json' },
  { phase: 3, text: 'Get auth token' },
  { phase: 4, text: 'Get Calendar Events' },
  { phase: 13, text: 'Checking Google Login' }
];

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

    .keyboard-overlay {
      position: fixed;
      inset: 0;
      background-color: var(--bg-color, #202124);
      z-index: 2000;
      display: flex;
      flex-direction: column;
      align-items: center;
      animation: fadeIn 0.2s cubic-bezier(0.2, 0, 0, 1);
    }

    .overlay-input-container {
      display: flex;
      align-items: center;
      background-color: var(--card-bg, #28292c);
      padding: 0 24px;
      height: 64px;
      border-radius: 32px;
      gap: 16px;
      width: calc(100% - 32px);
      max-width: 768px;
      margin-top: 24px;
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
      animation: slideDown 0.3s cubic-bezier(0.2, 0, 0, 1);
      box-sizing: border-box;
    }

    .overlay-input {
      background: transparent;
      border: none;
      color: var(--text-primary, #e8eaed);
      font-size: 1.5rem;
      font-family: monospace;
      outline: none;
      width: 100%;
    }

    .keyboard-popup {
      margin-top: auto;
      margin-bottom: 12px;
      width: 100%;
      max-width: 800px;
      display: flex;
      justify-content: center;
      animation: slideUp 0.4s cubic-bezier(0.2, 0, 0, 1);
    }

    @keyframes fadeIn {
      from { opacity: 0; }
      to { opacity: 1; }
    }
    @keyframes slideUp {
      from { transform: translateY(100%); opacity: 0; }
      to { transform: translateY(0); opacity: 1; }
    }
    @keyframes slideDown {
      from { transform: translateY(-50%); opacity: 0; }
      to { transform: translateY(0); opacity: 1; }
    }
  `;

  @state() private showChecklist = false;
  
  @state() private setupPhase = 0;
  @state() private phase = '';
  @state() private currentNode = '';
  
  @state() private isLoading = true;
  @state() private statusPageText = 'Initializing...';
  @state() private extraHtml: TemplateResult | undefined = undefined;
  
  @state() private lastProcessedNode = '';
  
  @state() private allClear = false;
  @state() private countdown = 15;
  @state() private activeKeyboardInput: 'token' | 'password' | null = null;
  @state() private showKeyboardInputText = false;
  @state() private keyboardInputValue = '';

  private openKeyboard(type: 'token' | 'password', target: any) {
    this.activeKeyboardInput = type;
    this.showKeyboardInputText = false;
    this.keyboardInputValue = target.value || '';
  }

  private handleKeyPress(e: CustomEvent<KeyPressedEvent>) {
    const key = e.detail.key;
    const inputId = this.activeKeyboardInput === 'token' ? '#token-input' : '#google-password-input';
    const input = this.shadowRoot?.querySelector(inputId) as any;
    if (!input) return;

    if (key === 'Dismiss') {
      this.activeKeyboardInput = null;
      return;
    }
    
    if (key === 'Enter') {
      this.activeKeyboardInput = null;
      if (inputId === '#token-input') this.handleTokenSubmit();
      if (inputId === '#google-password-input') this.handlePasswordSubmit();
      return;
    }

    let val = this.keyboardInputValue;
    if (key === 'Backspace') {
      val = val.slice(0, -1);
    } else if (key === 'Left' || key === 'Right') {
      // not easily handled without selectionStart
    } else {
      val += key;
    }
    input.value = val;
    this.keyboardInputValue = val;
  }

  connectedCallback() {
    super.connectedCallback();
    
    // Trigger the M3 transition to reveal the checklist shortly after load
    getAppClock().setTimeout(async () => {
      this.showChecklist = true;
    }, 1200);

    wsClient.onStateUpdate((state) => {
      if (state.current_node) this.currentNode = state.current_node;
      if (state.phase) this.phase = state.phase;
      if (state.setup_phase !== undefined) this.setupPhase = state.setup_phase;
      
      this.isLoading = (this.phase === 'work' || this.phase === 'pre-work' || this.phase === 'transitioning' || this.phase === 'pre-setup' || this.phase === 'setup');
      
      if (this.setupPhase === 1000) {
        if (!this.allClear) {
           this.allClear = true;
           this.startCountdown();
        }
        return;
      }
      
      this.processStateUpdate();
    });
  }

  private async processStateUpdate() {
    if (this.currentNode === "Credentials Phase") {
      this.statusPageText = 'Upload Credentials to display';
      if (this.lastProcessedNode !== this.currentNode) {
        this.lastProcessedNode = this.currentNode;
        
        let hostIp = window.location.host;
        const ipRes = await WrapPromise(wsClient.request({ type: 'get_ip' }), 'Failed to fetch IP');
        if (ipRes.ok) {
          const ipData = ipRes.safeUnwrap();
          if (ipData && ipData.ip) {
            const port = window.location.port ? `:${window.location.port}` : '';
            hostIp = `${ipData.ip}${port}`;
          }
        }
        
        const uploadUrl = `http://${hostIp}/upload.html`;
        const qrRes = await WrapPromise(QRCode.toDataURL(uploadUrl, {
          margin: 2, width: 180, color: { dark: '#000000', light: '#ffffff' }
        }), 'failed to generate qr');

        if (qrRes.ok) {
          this.extraHtml = html`
            <div style="display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; margin-top: 8px; flex-grow: 1; min-height: 0;">
              <img src="${qrRes.safeUnwrap()}" alt="QR Code to Upload Page" style="border-radius: 8px; max-height: 100%; min-height: 0; object-fit: contain;" />
              <a href="${uploadUrl}" style="color: #a8c7fa; text-decoration: none; word-break: break-all; text-align: center; font-size: 1rem; flex-shrink: 0;">${uploadUrl}</a>
            </div>
          `;
        }
      }
    } else if (this.currentNode === "Auth Token Phase") {
      this.statusPageText = 'Authorize the display';
      if (this.lastProcessedNode !== this.currentNode) {
        this.lastProcessedNode = this.currentNode;
        
        const authUrlRes = await WrapPromise(wsClient.request({ type: 'get_auth_url' }), 'Failed to fetch auth url');
        if (authUrlRes.ok) {
          const authUrlData = authUrlRes.safeUnwrap();
          if (authUrlData && authUrlData.url) {
            const url = authUrlData.url;
            const qrRes = await WrapPromise(QRCode.toDataURL(url, {
              margin: 2, width: 180, color: { dark: '#000000', light: '#ffffff' }
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
                    <md-outlined-text-field id="token-input" label="Paste URL or Code" style="flex-grow: 1;" @focus=${(e: Event) => this.openKeyboard('token', e.target)}></md-outlined-text-field>
                    <md-filled-button @click=${this.handleTokenSubmit}>Submit</md-filled-button>
                  </div>
                </div>
              `;
            }
          }
        }
      }
    } else if (this.currentNode === "Password Input Page") {
      this.statusPageText = 'Google requires your password';
      this.extraHtml = html`
        <div style="display: flex; flex-direction: column; gap: 8px;">
           <div style="font-size: 0.9rem;">Please enter your Google account password to proceed.</div>
           <div style="display: flex; gap: 8px; align-items: center; margin-bottom: 8px;">
             <md-outlined-text-field id="google-password-input" label="Google Password" type="password" style="flex-grow: 1;" @focus=${(e: Event) => this.openKeyboard('password', e.target)}></md-outlined-text-field>
             <md-filled-button @click=${this.handlePasswordSubmit}>Submit</md-filled-button>
           </div>
        </div>
      `;
      this.lastProcessedNode = this.currentNode;
    } else if (this.currentNode === "Init Server") {
      this.statusPageText = 'Server Initializing...';
      const hasWifi = await this.checkStage1();
      if (!hasWifi) {
          this.statusPageText = 'Please Connect this Pi to the internet';
          this.extraHtml = html`<pre><code>mvda-lounge-display-wifi-portal</code></pre>`;
      } else {
          this.extraHtml = undefined;
      }
      this.lastProcessedNode = this.currentNode;
    } else {
      this.statusPageText = `Currently in ${this.currentNode || 'Setup'} (${this.phase || 'Waiting'})`;
      this.extraHtml = undefined;
      this.lastProcessedNode = this.currentNode;
    }
  }

  private async handleTokenSubmit() {
    if (!this.shadowRoot) return;
    const input = this.shadowRoot.querySelector('#token-input') as HTMLInputElement;
    if (!input || !input.value) return;

    const token = input.value;
    const res = await WrapPromise(wsClient.request({ type: 'submit_token', payload: { code: token } }), 'Failed to post token');

    if (res.ok && res.safeUnwrap().status === 'ok') {
      input.value = '';
      this.statusPageText = 'Token submitted, processing...';
    } else {
      alert('Failed to submit token');
    }
  }

  private async handlePasswordSubmit() {
    if (!this.shadowRoot) return;
    const input = this.shadowRoot.querySelector('#google-password-input') as HTMLInputElement;
    if (!input || !input.value) return;

    const password = input.value;
    const res = await WrapPromise(wsClient.request({ type: 'submit_password', payload: { password } }), 'Failed to post password');

    if (res.ok && res.safeUnwrap().status === 'ok') {
      input.value = '';
      this.statusPageText = 'Password submitted, processing...';
    } else {
      alert('Failed to submit password');
    }
  }

  private async checkStage1(): Promise<boolean> {
    const res = await WrapPromise(wsClient.request({ type: 'has_wifi' }), 'failed fetch');
    if (!res.ok) return false;
    
    const data = res.safeUnwrap();
    if (!data || !data.internetAccess) return false;
    
    return true;
  }

  private countdownTimer?: number;

  private startCountdown() {
    this.countdown = 15;
    const scheduleCountdown = () => {
      this.countdownTimer = getAppClock().setTimeout(async () => {
        this.countdown--;
        this.statusPageText = `Setup was successfull refreshing in ${this.countdown}`;
        if (this.countdown <= 0) {
          this.reloadPage();
        } else {
          scheduleCountdown();
        }
      }, 1000);
    };
    scheduleCountdown();
  }

  // Exposed for testing
  protected reloadPage() {
    window.location.reload();
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    if (this.countdownTimer) {
      getAppClock().clearTimeout(this.countdownTimer);
    }
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
          
          ${SETUP_STEPS.map(step => {
            const isCompleted = this.setupPhase > step.phase;
            // For step 13 (Google Login), it's active if setupPhase is between 5 and 13
            const isActive = step.phase === 13 
              ? (this.setupPhase >= 5 && this.setupPhase <= 13)
              : this.setupPhase === step.phase;
            
            return html`
              <div class="stage-container ${isCompleted ? 'completed-hide' : ''}">
                <label class="check-item">
                  <md-checkbox ?checked=${isCompleted} disabled></md-checkbox>
                  <span>${step.text}</span>
                </label>
                <div class="extra-html-container ${isActive && this.extraHtml ? 'open' : ''}">
                  ${isActive ? this.extraHtml : ''}
                </div>
              </div>
            `;
          })}

          ${this.setupPhase === 1000 ? html`
            <div class="check-item" style="justify-content: center; font-size: 1.3rem; margin-top: 20px; text-align: center; text-wrap: wrap;">
              Setup was successful! Redirecting...
            </div>
          ` : ''}
        </div>
      </div>

      ${this.activeKeyboardInput ? html`
        <div class="keyboard-overlay" @pointerdown=${(e: Event) => {
          if (e.target === e.currentTarget) this.activeKeyboardInput = null;
        }}>
          <div class="overlay-input-container">
            <div style="cursor: pointer; padding: 8px; display: flex; color: var(--text-secondary, #9aa0a6);" @click=${() => this.showKeyboardInputText = !this.showKeyboardInputText}>
              ${this.showKeyboardInputText 
                 ? html`<svg width="28" height="28" viewBox="0 0 24 24" fill="currentColor">
                     <path d="M12 4.5C7 4.5 2.73 7.61 1 12c1.73 4.39 6 7.5 11 7.5s9.27-3.11 11-7.5c-1.73-4.39-6-7.5-11-7.5zM12 17c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5zm0-8c-1.66 0-3 1.34-3 3s1.34 3 3 3 3-1.34 3-3-1.34-3-3-3z"/>
                   </svg>`
                 : html`<svg width="28" height="28" viewBox="0 0 24 24" fill="currentColor">
                     <path d="M12 7c2.76 0 5 2.24 5 5 0 .65-.13 1.26-.36 1.83l2.92 2.92c1.51-1.26 2.7-2.89 3.43-4.75-1.73-4.39-6-7.5-11-7.5-1.4 0-2.74.25-3.98.7l2.16 2.16C10.74 7.13 11.35 7 12 7zM2 4.27l2.28 2.28.46.46C3.08 8.3 1.78 10.02 1 12c1.73 4.39 6 7.5 11 7.5 1.55 0 3.03-.3 4.38-.84l.42.42L19.73 22 21 20.73 3.27 3 2 4.27zM7.53 9.8l1.55 1.55c-.05.21-.08.43-.08.65 0 1.66 1.34 3 3 3 .22 0 .44-.03.65-.08l1.55 1.55c-.67.33-1.41.53-2.2.53-2.76 0-5-2.24-5-5 0-.79.2-1.53.53-2.2zm4.31-.78l3.15 3.15.02-.16c0-1.66-1.34-3-3-3l-.17.01z"/>
                   </svg>`
              }
            </div>
            <input 
              type=${this.showKeyboardInputText ? 'text' : 'password'} 
              class="overlay-input"
              .value=${this.keyboardInputValue}
              @input=${(e: Event) => {
                const target = e.target as HTMLInputElement;
                this.keyboardInputValue = target.value;
                const inputId = this.activeKeyboardInput === 'token' ? '#token-input' : '#google-password-input';
                const input = this.shadowRoot?.querySelector(inputId) as HTMLInputElement;
                if (input) input.value = target.value;
              }}
            >
          </div>
          <div class="keyboard-popup">
            <virtual-keyboard @key-pressed=${this.handleKeyPress}></virtual-keyboard>
          </div>
        </div>
      ` : ''}
    `;
  }
}
