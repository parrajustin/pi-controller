import { LitElement, html, css } from 'lit';
import { customElement } from 'lit/decorators.js';

export interface KeyPressedEvent {
  key: string;
}

@customElement('virtual-keyboard')
export class VirtualKeyboard extends LitElement {
  static styles = css`
    :host {
      display: block;
      background-color: var(--card-bg, #28292c);
      padding: 16px;
      border-radius: 16px;
      box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
      width: 100%;
      max-width: 800px;
      user-select: none;
    }

    .row {
      display: flex;
      justify-content: center;
      gap: 8px;
      margin-bottom: 8px;
    }
    .row:last-child {
      margin-bottom: 0;
    }

    .key {
      background-color: var(--input-bg, #303134);
      border: 1px solid var(--card-border, #3c4043);
      border-radius: 8px;
      height: 56px;
      min-width: 48px;
      flex: 1;
      max-width: 64px;
      display: flex;
      justify-content: center;
      align-items: center;
      font-size: 1.25rem;
      font-weight: 500;
      color: var(--text-primary, #e8eaed);
      cursor: pointer;
      box-shadow: 0 1px 2px rgba(0, 0, 0, 0.1);
      transition: background-color 0.1s;
      -webkit-tap-highlight-color: transparent;
    }
    .key:hover {
      background-color: var(--card-border, #3c4043);
    }
    .key:active {
      background-color: var(--btn-bg, #494c50);
      box-shadow: none;
      transform: translateY(1px);
    }

    .key.wide {
      flex: 1.5;
      max-width: 80px;
    }

    .key.space {
      flex: 5;
      max-width: 400px;
    }

    .key.blue {
      background-color: #1a73e8;
      color: white;
      border-color: #1a73e8;
    }
    .key.blue:hover {
      background-color: #1557b0;
    }
    .key.blue:active {
      background-color: #174ea6;
    }
  `;

  private handleKeyClick(key: string) {
    this.dispatchEvent(
      new CustomEvent<KeyPressedEvent>('key-pressed', {
        detail: { key },
        bubbles: true,
        composed: true,
      }),
    );
  }

  render() {
    const row1 = ['q', 'w', 'e', 'r', 't', 'y', 'u', 'i', 'o', 'p', 'Backspace'];
    const row2 = ['a', 's', 'd', 'f', 'g', 'h', 'j', 'k', 'l', 'Enter'];
    const row3 = ['z', 'x', 'c', 'v', 'b', 'n', 'm', '-', 'Left', 'Right'];

    const row4 = ['Dismiss', ' '];

    return html`
      <div class="row">${row1.map((key) => this.renderKey(key))}</div>
      <div class="row">${row2.map((key) => this.renderKey(key))}</div>
      <div class="row">${row3.map((key) => this.renderKey(key))}</div>
      <div class="row">${row4.map((key) => this.renderKey(key))}</div>
    `;
  }

  private renderKey(key: string) {
    let label: string | import('lit').TemplateResult = key;
    let classes = 'key';

    if (key === 'Backspace') {
      label = '⌫';
      classes += ' wide';
    } else if (key === 'Enter') {
      label = '↵';
      classes += ' wide blue';
    } else if (key === 'Left') {
      label = '←';
    } else if (key === 'Right') {
      label = '→';
    } else if (key === ' ') {
      label = '';
      classes += ' space';
    } else if (key === 'Dismiss') {
      label = html`<svg width="24" height="24" viewBox="0 0 24 24" fill="currentColor">
        <path
          d="M20 5H4c-1.1 0-1.99.9-1.99 2L2 15c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V7c0-1.1-.9-2-2-2zm-9 3h2v2h-2V8zm0 3h2v2h-2v-2zM8 8h2v2H8V8zm0 3h2v2H8v-2zm-1 2H5v-2h2v2zm0-3H5V8h2v2zm9 7H8v-2h8v2zm0-4h-2v-2h2v2zm0-3h-2V8h2v2zm3 3h-2v-2h2v2zm0-3h-2V8h2v2zm-7 15l4-4H8l4 4z"
        />
      </svg>`;
      classes += ' wide';
    }

    return html` <div class="${classes}" @click=${() => this.handleKeyClick(key)}>${label}</div> `;
  }
}
