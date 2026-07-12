import { LitElement, html, css } from 'lit';
import { customElement, state, query } from 'lit/decorators.js';
import '@material/web/icon/icon.js';
import '@material/web/iconbutton/icon-button.js';
import '@material/web/button/filled-button.js';
import '@material/web/textfield/outlined-text-field.js';
import { wsClient } from '../ws-client.js';

@customElement('touchpad-controller')
export class TouchpadController extends LitElement {
  @state()
  private currentUrl = '';

  @state()
  private keyboardActive = false;

  @query('#url-input')
  urlInput!: HTMLInputElement;

  @query('#keyboard-input')
  keyboardInput!: HTMLInputElement;

  private pollInterval?: number;
  private lastTouchX = 0;
  private lastTouchY = 0;
  private lastScrollY = 0;

  static styles = css`
    :host {
      display: flex;
      flex-direction: column;
      width: 100%;
      height: 100%;
      background-color: var(--bg-color, #202124);
      color: var(--text-primary, #e8eaed);
      padding: 16px;
      box-sizing: border-box;
      gap: 16px;
    }

    .top-bar {
      display: flex;
      align-items: center;
      gap: 12px;
      width: 100%;
    }

    .url-input {
      flex: 1;
      padding: 12px;
      border-radius: 24px;
      border: 1px solid var(--card-border, #3c4043);
      background-color: var(--input-bg, #303134);
      color: var(--text-primary, #e8eaed);
      font-size: 16px;
      outline: none;
    }

    .nav-buttons {
      display: flex;
      align-items: center;
      gap: 8px;
    }

    .keyboard-section {
      display: flex;
      align-items: center;
      gap: 12px;
    }

    .keyboard-input {
      flex: 1;
      padding: 12px;
      border-radius: 8px;
      border: 1px solid var(--card-border, #3c4043);
      background-color: var(--input-bg, #303134);
      color: var(--text-primary, #e8eaed);
      font-size: 16px;
      outline: none;
    }

    .mouse-controls {
      display: flex;
      flex-direction: column;
      flex: 1;
      gap: 12px;
      background-color: #28292c;
      border-radius: 12px;
      padding: 12px;
    }

    .mouse-buttons {
      display: flex;
      height: 60px;
      gap: 12px;
    }

    .mouse-btn {
      flex: 1;
      background-color: #3c4043;
      border-radius: 8px;
      display: flex;
      align-items: center;
      justify-content: center;
      cursor: pointer;
      user-select: none;
    }

    .mouse-btn:active {
      background-color: #5f6368;
    }

    .scroll-pad {
      flex: 1;
      background-color: #303134;
      border-radius: 8px;
      display: flex;
      align-items: center;
      justify-content: center;
      cursor: ns-resize;
      touch-action: none;
    }

    .touch-pad {
      flex: 1;
      background-color: #f1f3f4; /* White-ish pad to replicate a touchpad */
      border-radius: 12px;
      touch-action: none; /* Prevent browser scrolling */
      display: flex;
      align-items: center;
      justify-content: center;
      color: #202124;
      font-weight: bold;
      user-select: none;
    }

    .footer {
      display: flex;
      justify-content: flex-start;
    }
  `;

  connectedCallback() {
    super.connectedCallback();
    this.fetchUrl();
    this.pollInterval = window.setInterval(() => this.fetchUrl(), 3000);
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    if (this.pollInterval) {
      window.clearInterval(this.pollInterval);
    }
  }

  private async fetchUrl() {
    try {
      const res = await wsClient.request({ type: 'get_url' });
      if (res.ok && res.val && res.val.url) {
        if (this.urlInput && document.activeElement !== this.urlInput) {
          this.currentUrl = res.val.url;
          this.urlInput.value = this.currentUrl;
        }
      }
    } catch (e) {
      console.error('Failed to fetch URL', e);
    }
  }

  private handleUrlKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      const newUrl = (e.target as HTMLInputElement).value;
      wsClient.request({ type: 'navigate_url', payload: { url: newUrl } });
      this.urlInput.blur();
    }
  }

  private handleNavClick(action: string) {
    wsClient.request({ type: action });
    if (action === 'history_back' || action === 'history_forward') {
      setTimeout(() => this.fetchUrl(), 500);
    } else if (action === 'refresh') {
      wsClient.request({ type: 'navigate_url', payload: { url: this.currentUrl } });
    }
  }

  private toggleKeyboard() {
    this.keyboardActive = !this.keyboardActive;
    if (this.keyboardActive) {
      setTimeout(() => this.keyboardInput?.focus(), 100);
    }
  }

  private handleKeyboardInput(e: Event) {
    const target = e.target as HTMLInputElement;
    const val = target.value;
    if (val.length > 0) {
      // Send the last character typed
      const key = val.substring(val.length - 1);
      wsClient.request({ type: 'keyboard_input', payload: { keys: key } });
    }
  }

  private handleKeyboardKeydown(e: KeyboardEvent) {
    if (e.key === 'Backspace') {
      wsClient.request({ type: 'keyboard_input', payload: { keys: '\\b' } });
      e.preventDefault();
    } else if (e.key === 'Enter') {
      wsClient.request({ type: 'keyboard_input', payload: { keys: '\\n' } });
      (e.target as HTMLInputElement).value = '';
      e.preventDefault();
    }
  }

  private handleMouseClick(button: string) {
    wsClient.request({ type: 'mouse_click', payload: { button } });
  }

  private handleTouchStart(e: TouchEvent) {
    if (e.touches.length > 0) {
      this.lastTouchX = e.touches[0].clientX;
      this.lastTouchY = e.touches[0].clientY;
    }
  }

  private handleTouchMove(e: TouchEvent) {
    e.preventDefault(); // Prevent scrolling
    if (e.touches.length > 0) {
      const currentX = e.touches[0].clientX;
      const currentY = e.touches[0].clientY;
      const deltaX = currentX - this.lastTouchX;
      const deltaY = currentY - this.lastTouchY;
      
      // Send mouse move
      wsClient.request({ type: 'mouse_move', payload: { deltaX, deltaY } });
      
      this.lastTouchX = currentX;
      this.lastTouchY = currentY;
    }
  }

  private handleScrollStart(e: TouchEvent) {
    if (e.touches.length > 0) {
      this.lastScrollY = e.touches[0].clientY;
    }
  }

  private handleScrollMove(e: TouchEvent) {
    e.preventDefault();
    if (e.touches.length > 0) {
      const currentY = e.touches[0].clientY;
      const deltaY = this.lastScrollY - currentY; // Invert for natural scrolling
      
      wsClient.request({ type: 'mouse_scroll', payload: { deltaY } });
      
      this.lastScrollY = currentY;
    }
  }

  private handleExit() {
    wsClient.request({ type: 'exit_control' });
  }

  render() {
    return html`
      <div class="top-bar">
        <input 
          id="url-input"
          class="url-input" 
          type="text" 
          .value=${this.currentUrl}
          @keydown=${this.handleUrlKeydown}
          placeholder="Enter URL..."
        />
      </div>

      <div class="nav-buttons">
        <md-icon-button @click=${() => this.handleNavClick('history_back')}><md-icon>arrow_back</md-icon></md-icon-button>
        <md-icon-button @click=${() => this.handleNavClick('history_forward')}><md-icon>arrow_forward</md-icon></md-icon-button>
        <md-icon-button @click=${() => this.handleNavClick('refresh')}><md-icon>refresh</md-icon></md-icon-button>
        <md-icon-button @click=${this.toggleKeyboard}><md-icon>keyboard</md-icon></md-icon-button>
      </div>

      ${this.keyboardActive ? html`
        <div class="keyboard-section">
          <input 
            id="keyboard-input"
            class="keyboard-input"
            type="text"
            placeholder="Type to send to screen..."
            @input=${this.handleKeyboardInput}
            @keydown=${this.handleKeyboardKeydown}
          />
        </div>
      ` : ''}

      <div class="mouse-controls">
        <div class="mouse-buttons">
          <div class="mouse-btn" @click=${() => this.handleMouseClick('left')}><md-icon>left_click</md-icon></div>
          <div class="scroll-pad" 
               @touchstart=${this.handleScrollStart} 
               @touchmove=${this.handleScrollMove}>
            <md-icon>unfold_more</md-icon>
          </div>
          <div class="mouse-btn" @click=${() => this.handleMouseClick('right')}><md-icon>right_click</md-icon></div>
        </div>
        <div class="touch-pad"
             @touchstart=${this.handleTouchStart}
             @touchmove=${this.handleTouchMove}
             @click=${() => this.handleMouseClick('left')}>
          TOUCHPAD
        </div>
      </div>

      <div class="footer">
        <md-filled-button @click=${this.handleExit}>
          <md-icon slot="icon">exit_to_app</md-icon>
          Exit Control
        </md-filled-button>
      </div>
    `;
  }
}
