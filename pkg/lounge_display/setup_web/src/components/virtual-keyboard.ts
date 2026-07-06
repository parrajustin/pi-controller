import { LitElement, html, css } from 'lit';
import { customElement, state } from 'lit/decorators.js';

export interface KeyPressedEvent {
  key: string;
}

@customElement('virtual-keyboard')
export class VirtualKeyboard extends LitElement {
  static styles = css`
    :host {
      display: block;
      background-color: #f1f3f4;
      padding: 8px;
      border-radius: 16px 16px 0 0;
      box-shadow: 0 -4px 16px rgba(0, 0, 0, 0.2);
      width: 100%;
      max-width: 800px;
      user-select: none;
      box-sizing: border-box;
      position: absolute;
      bottom: 0;
      left: 50%;
      transform: translateX(-50%);
      z-index: 1000;
    }

    .row {
      display: flex;
      justify-content: center;
      gap: 6px;
      margin-bottom: 6px;
    }
    .row:last-child {
      margin-bottom: 0;
    }

    .key {
      background-color: #ffffff;
      border: 1px solid #dadce0;
      border-radius: 6px;
      height: 48px;
      min-width: 28px;
      flex: 1;
      max-width: 52px;
      display: flex;
      justify-content: center;
      align-items: center;
      font-size: 1.1rem;
      font-weight: 500;
      color: #3c4043;
      cursor: pointer;
      box-shadow: 0 1px 2px rgba(0, 0, 0, 0.1);
      transition: background-color 0.1s;
      -webkit-tap-highlight-color: transparent;
    }
    .key:hover {
      background-color: #f8f9fa;
    }
    .key:active {
      background-color: #e8eaed;
      box-shadow: none;
      transform: translateY(1px);
    }

    .key.wide {
      flex: 1.5;
      max-width: 70px;
    }
    
    .key.extra-wide {
      flex: 2;
      max-width: 90px;
    }

    .key.space {
      flex: 5;
      max-width: 300px;
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

    .key.active-shift {
      background-color: #e8eaed;
      box-shadow: inset 0 2px 4px rgba(0,0,0,0.1);
    }
    .key.caps-lock {
      background-color: #1a73e8;
      color: white;
      border-color: #1a73e8;
    }
  `;

  // 0: off, 1: single char, 2: caps lock
  @state() private shiftState: 0 | 1 | 2 = 0;
  private lastShiftClick = 0;

  private shiftMap: Record<string, string> = {
    '`': '~', '1': '!', '2': '@', '3': '#', '4': '$', '5': '%', 
    '6': '^', '7': '&', '8': '*', '9': '(', '0': ')', '-': '_', '=': '+',
    '[': '{', ']': '}', '\\': '|', ';': ':', "'": '"', ',': '<', '.': '>', '/': '?'
  };

  private handleKeyClick(key: string) {
    if (key === 'Shift') {
      const now = Date.now();
      if (now - this.lastShiftClick < 400) {
        // Double tap
        this.shiftState = 2;
      } else {
        if (this.shiftState === 2 || this.shiftState === 1) {
          this.shiftState = 0;
        } else {
          this.shiftState = 1;
        }
      }
      this.lastShiftClick = now;
      return;
    }

    let outKey = key;
    if (this.shiftState > 0 && key.length === 1) {
      if (this.shiftMap[key]) {
        outKey = this.shiftMap[key];
      } else {
        outKey = key.toUpperCase();
      }
    }
    
    this.dispatchEvent(
      new CustomEvent<KeyPressedEvent>('key-pressed', {
        detail: { key: outKey },
        bubbles: true,
        composed: true,
      }),
    );

    // Turn off single shift after typing a character (that is not Dismiss/Backspace/Enter etc)
    if (this.shiftState === 1 && key.length === 1) {
      this.shiftState = 0;
    }
  }

  render() {
    const row1 = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '0', '-', '=', 'Backspace'];
    const row2 = ['q', 'w', 'e', 'r', 't', 'y', 'u', 'i', 'o', 'p', '[', ']', '\\'];
    const row3 = ['a', 's', 'd', 'f', 'g', 'h', 'j', 'k', 'l', ';', "'", 'Enter'];
    const row4 = ['Shift', 'z', 'x', 'c', 'v', 'b', 'n', 'm', ',', '.', '/', 'Shift'];
    const row5 = ['Dismiss', ' '];

    return html`
      <div class="row">${row1.map((key) => this.renderKey(key))}</div>
      <div class="row">${row2.map((key) => this.renderKey(key))}</div>
      <div class="row">${row3.map((key) => this.renderKey(key))}</div>
      <div class="row">${row4.map((key) => this.renderKey(key))}</div>
      <div class="row">${row5.map((key) => this.renderKey(key))}</div>
    `;
  }

  private renderKey(key: string) {
    let displayKey = key;
    if (this.shiftState > 0 && key.length === 1) {
      if (this.shiftMap[key]) {
        displayKey = this.shiftMap[key];
      } else {
        displayKey = key.toUpperCase();
      }
    }

    let label: string | import('lit').TemplateResult = displayKey;
    let classes = 'key';

    if (key === 'Backspace') {
      label = '⌫';
      classes += ' extra-wide';
    } else if (key === 'Enter') {
      label = '↵';
      classes += ' extra-wide blue';
    } else if (key === 'Shift') {
      label = '⇧';
      classes += ' wide';
      if (this.shiftState === 1) classes += ' active-shift';
      if (this.shiftState === 2) classes += ' caps-lock';
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
      classes += ' extra-wide';
    }

    return html` <div class="${classes}" @click=${() => this.handleKeyClick(key)}>${label}</div> `;
  }
}
