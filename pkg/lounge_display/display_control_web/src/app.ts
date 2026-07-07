import { LitElement, html, css } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import './components/setup-display.js';
import './components/display-controller.js';
import { WrapPromise } from 'standard-ts-lib/src/wrap_promise.js';

@customElement('display-control-app')
export class DisplayControlApp extends LitElement {
  @state()
  private setupCompleted = false;

  async connectedCallback() {
    super.connectedCallback();
    this.pollStatus();
  }

  private async pollStatus() {
    while (true) {
      const res = await WrapPromise(fetch('/api/status'), 'failed fetch');
      if (res.ok && res.safeUnwrap().ok) {
        const data = await WrapPromise(res.safeUnwrap().json(), 'failed json');
        if (data.ok && data.safeUnwrap().status === 'ready') {
          if (!this.setupCompleted) {
            this.setupCompleted = true;
          }
          await new Promise(r => setTimeout(r, 5000));
          continue;
        } else {
          this.setupCompleted = false;
        }
      }
      await new Promise(r => setTimeout(r, 2000));
    }
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

