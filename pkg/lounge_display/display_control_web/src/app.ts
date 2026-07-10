import { LitElement, html, css } from 'lit';
import { customElement } from 'lit/decorators.js';
import './components/display-controller.js';

@customElement('display-control-app')
export class DisplayControlApp extends LitElement {
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
      <display-controller style="flex-grow: 1;"></display-controller>
    `;
  }
}
