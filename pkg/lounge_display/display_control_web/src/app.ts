import { LitElement, html, css } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import './components/setup-display.js';
import './components/display-controller.js';
import { wsClient } from './ws-client.js';

@customElement('display-control-app')
export class DisplayControlApp extends LitElement {
  @state()
  private setupCompleted = false;

  async connectedCallback() {
    super.connectedCallback();
    try {
      const res = await fetch('/api/setup_done');
      if (res.ok) {
        const data = await res.json();
        this.setupCompleted = data.setup_ready === true;
      }
    } catch (e) {
      console.error('Failed to fetch setup status', e);
    }

    wsClient.onStateUpdate((state) => {
      this.setupCompleted = state.setup_ready === true;
    });
  }

  static styles = css`
    :host {
      display: flex;
      width: 100%;
      height: 100%;
      background-color: var(--bg-color, #202124);
    }
  `;

  render() {
    return html`
      ${this.setupCompleted ? html`
        <display-controller style="flex-grow: 1;"></display-controller>
      ` : html`
        <setup-display style="flex-grow: 1;"></setup-display>
      `}
    `;
  }
}

