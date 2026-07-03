import { LitElement, html, css } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import '@material/web/icon/icon.js';
import '@material/web/iconbutton/icon-button.js';
import '@material/web/divider/divider.js';

@customElement('display-control-app')
export class DisplayControlApp extends LitElement {
  @state()
  private isOpen = false;

  static styles = css`
    :host {
      display: flex;
      width: 100%;
      height: 100%;
      background-color: var(--md-sys-color-background, #f5f5f5);
    }

    .container {
      display: flex;
      width: 100%;
      height: 100%;
    }

    .sidebar {
      position: relative;
      display: flex;
      flex-direction: column;
      background-color: var(--md-sys-color-outline-variant, #cac4d0);
      transition: width 0.3s ease, background-color 0.3s ease;
      width: 5px;
      z-index: 5;
    }

    .sidebar.open {
      width: 250px;
      background-color: var(--md-sys-color-surface-container, #e8def8);
      border-right: 1px solid var(--md-sys-color-outline-variant, #cac4d0);
    }

    .toggle-btn {
      position: absolute;
      top: 50%;
      transform: translateY(-50%);
      right: -24px;
      width: 24px;
      height: 48px;
      z-index: 10;
      background-color: var(--md-sys-color-outline-variant, #cac4d0);
      border-radius: 0 24px 24px 0;
      box-shadow: 2px 0 4px rgba(0,0,0,0.15);
      border: none;
      padding: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      cursor: pointer;
      color: var(--md-sys-color-on-surface-variant, #49454f);
      transition: background-color 0.3s ease;
      outline: none;
    }

    .sidebar.open .toggle-btn {
      background-color: var(--md-sys-color-surface-container, #e8def8);
    }

    .toggle-btn:hover {
      background-color: var(--md-sys-color-secondary-container, #e8def8);
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
      color: var(--md-sys-color-on-surface, #1d1b20);
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
      color: var(--md-sys-color-on-surface-variant, #49454f);
      border-radius: 28px;
      margin: 0 8px;
      transition: background-color 0.2s;
    }

    .nav-item:hover {
      background-color: var(--md-sys-color-secondary-container, #e8def8);
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
    }

    iframe {
      width: 100%;
      height: 100%;
      border: none;
    }
  `;

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
          <iframe src="http://localhost:8080" allow="camera; microphone; display-capture; fullscreen"></iframe>
        </div>
      </div>
    `;
  }
}
