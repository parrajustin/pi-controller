import { LitElement, html, css } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import '@material/web/icon/icon.js';
import '@material/web/iconbutton/icon-button.js';
import '@material/web/divider/divider.js';
import './lounge-display.js';
import './setup-display.js';
import { wsClient } from '../ws-client.js';
import { WrapPromise } from 'standard-ts-lib/src/wrap_promise.js';

@customElement('display-controller')
export class DisplayController extends LitElement {
  @state()
  private isOpen = false;

  @state()
  private serverState: Record<string, any> = {};

  @state()
  private setupCompleted = false;

  private unsubscribe?: () => void;

  static styles = css`
    :host {
      display: flex;
      width: 100%;
      height: 100%;
      background-color: var(--bg-color, #202124);
    }

    .container {
      display: flex;
      width: 100%;
      height: 100%;
      position: relative;
    }

    .sidebar {
      position: absolute;
      left: 0;
      top: 0;
      height: 100%;
      display: flex;
      flex-direction: column;
      background-color: var(--card-bg, #28292c);
      transition: width 0.3s ease, background-color 0.3s ease;
      width: 5px;
      z-index: 5;
    }

    .sidebar.hidden {
      display: none;
    }

    .sidebar.open {
      width: 250px;
      background-color: var(--input-bg, #303134);
      border-right: 1px solid var(--card-border, #3c4043);
    }

    .toggle-btn {
      position: absolute;
      top: 50%;
      transform: translateY(-50%);
      right: -24px;
      width: 24px;
      height: 48px;
      z-index: 10;
      background-color: var(--card-bg, #28292c);
      border-radius: 0 24px 24px 0;
      box-shadow: 2px 0 4px rgba(0,0,0,0.5);
      border: none;
      padding: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      cursor: pointer;
      color: var(--text-primary, #e8eaed);
      transition: background-color 0.3s ease;
      outline: none;
    }

    .sidebar.open .toggle-btn {
      background-color: var(--input-bg, #303134);
    }

    .toggle-btn:hover {
      background-color: var(--card-border, #3c4043);
    }

    .sidebar-clip {
      width: 100%;
      height: 100%;
      overflow: hidden;
    }

    .sidebar-content {
      width: 250px;
      height: 100%;
      display: flex;
      flex-direction: column;
      opacity: 0;
      pointer-events: none;
      transition: opacity 0.2s ease;
    }

    .sidebar.open .sidebar-content {
      opacity: 1;
      pointer-events: auto;
    }

    .sidebar-header {
      display: flex;
      align-items: center;
      padding: 8px 16px;
      min-height: 48px;
    }

    .app-title {
      font-size: 18px;
      font-weight: 500;
      white-space: nowrap;
      color: var(--text-primary, #e8eaed);
    }

    .nav-items {
      flex: 1;
      display: flex;
      flex-direction: column;
      gap: 8px;
      padding: 8px 0;
    }

    .nav-item {
      display: flex;
      align-items: center;
      padding: 8px 12px;
      cursor: pointer;
      color: var(--text-secondary, #9aa0a6);
      border-radius: 28px;
      margin: 0 8px;
      transition: background-color 0.2s;
    }

    .nav-item:hover {
      background-color: var(--card-border, #3c4043);
      color: var(--text-primary, #e8eaed);
    }

    .nav-label {
      margin-left: 12px;
      font-size: 14px;
      font-weight: 500;
      white-space: nowrap;
    }

    .main-content {
      flex: 1;
      display: flex;
      margin-left: 5px; /* Offset the 5px sidebar closed width */
    }
  `;

  async connectedCallback() {
    super.connectedCallback();
    
    this.unsubscribe = wsClient.onStateUpdate((state) => {
      this.serverState = state;
      this.setupCompleted = state.setup_ready === true;
    });

    const res = await WrapPromise(fetch('/api/setup_done'), 'Failed to fetch setup status');
    if (res.ok) {
      const data = res.safeUnwrap();
      if (data.ok) {
        const jsonRes = await WrapPromise(data.json(), 'Failed to fetch setup status');
        if (jsonRes.ok) {
          this.setupCompleted = jsonRes.safeUnwrap().setup_ready === true;
        } else {
          console.error('Failed to fetch setup status', jsonRes.val);
        }
      }
    } else {
      console.error('Failed to fetch setup status', res.val);
    }
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    if (this.unsubscribe) {
      this.unsubscribe();
    }
  }

  private toggleSidebar() {
    this.isOpen = !this.isOpen;
  }

  render() {
    return html`
      <div class="container">
        <div class="sidebar ${this.isOpen ? 'open' : ''}">
          <button class="toggle-btn" @click=${this.toggleSidebar}>
            <md-icon>${this.isOpen ? 'chevron_left' : 'chevron_right'}</md-icon>
          </button>
          <div class="sidebar-clip">
            <div class="sidebar-content">
              <div class="sidebar-header">
                <span class="app-title">Display Control</span>
              </div>
              <md-divider></md-divider>
              <div class="nav-items">
                <div class="nav-item">
                  <md-icon>settings</md-icon>
                  <span class="nav-label">Settings</span>
                </div>
                <div class="nav-item">
                  <md-icon>bug_report</md-icon>
                  <span class="nav-label">Test</span>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div class="main-content">
          ${this.setupCompleted ? html`
            <lounge-display style="flex-grow: 1;"></lounge-display>
          ` : html`
            <setup-display style="flex-grow: 1;"></setup-display>
          `}
        </div>
      </div>
    `;
  }
}
