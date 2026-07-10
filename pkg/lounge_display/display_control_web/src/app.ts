import { LitElement, html, css } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import './components/setup-display.js';
import './components/display-controller.js';
import { wsClient } from './ws-client.js';
import { WrapPromise } from 'standard-ts-lib/src/wrap_promise.js';

@customElement('display-control-app')
export class DisplayControlApp extends LitElement {
  @state()
  private setupCompleted = false;

  async connectedCallback() {
    super.connectedCallback();
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

