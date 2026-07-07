import { LitElement, html, css } from 'lit';
import { customElement } from 'lit/decorators.js';

@customElement('icon-button')
export class IconButton extends LitElement {
  static styles = css`
    .action-btn {
      background-color: var(--btn-bg);
      border: none;
      border-radius: 50%;
      width: 56px;
      height: 56px;
      display: flex;
      justify-content: center;
      align-items: center;
      cursor: pointer;
      color: var(--text-primary);
      transition: background-color 0.2s;
    }
    .action-btn:hover {
      background-color: #4a4d51;
    }
    ::slotted(svg) {
      width: 24px;
      height: 24px;
      fill: currentColor;
    }
  `;

  render() {
    return html`
      <button class="action-btn">
        <slot></slot>
      </button>
    `;
  }
}
