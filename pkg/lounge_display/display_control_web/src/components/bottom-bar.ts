import { LitElement, html, css } from 'lit';
import { customElement, state, query, property } from 'lit/decorators.js';
import './icon-button.js';
import './virtual-keyboard.js';
import { KeyPressedEvent } from './virtual-keyboard.js';
import { getAppClock } from '../clock-provider.js';

@customElement('bottom-bar')
export class BottomBar extends LitElement {
  @property({ type: Boolean }) isLoading = false;
  @state() private showKeyboard = false;
  @state() private inputValue = '';

  @query('.input-field') private inputField!: HTMLInputElement;
  @query('.overlay-input') private overlayInput!: HTMLInputElement;

  static styles = css`
    :host {
      display: block;
      transition: opacity 0.5s ease;
      opacity: 1;
    }
    :host([isloading]) {
      opacity: 0;
      pointer-events: none;
    }

    .bottom-bar {
      display: flex;
      justify-content: center;
      margin-top: 24px;
      flex-shrink: 0;
      position: relative;
    }
    .bottom-controls {
      display: flex;
      align-items: center;
      background-color: var(--bottom-bar-bg);
      padding: 16px;
      border-radius: 32px;
      gap: 16px;
    }
    .input-container {
      display: flex;
      align-items: center;
      background-color: var(--input-bg);
      padding: 0 24px;
      height: 56px;
      border-radius: 28px;
      gap: 12px;
      width: 320px;
    }
    .input-icon {
      color: var(--text-secondary);
    }
    .input-field {
      background: transparent;
      border: none;
      color: var(--text-primary);
      font-size: 1.1rem;
      font-family: var(--font-family);
      outline: none;
      width: 100%;
    }
    .input-field::placeholder {
      color: var(--text-secondary);
    }

    /* Full-screen Overlay Styles */
    .overlay {
      position: fixed;
      inset: 0;
      background-color: var(--bg-color);
      z-index: 1000;
      display: flex;
      flex-direction: column;
      align-items: center;
      animation: fadeIn 0.2s cubic-bezier(0.2, 0, 0, 1);
    }

    .overlay-input-container {
      display: flex;
      align-items: center;
      background-color: var(--input-bg);
      padding: 0 32px;
      height: 72px;
      border-radius: 36px;
      gap: 16px;
      width: 100%;
      max-width: 800px;
      margin-top: 48px;
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
      animation: slideDown 0.3s cubic-bezier(0.2, 0, 0, 1);
    }

    .overlay-input {
      background: transparent;
      border: none;
      color: var(--text-primary);
      font-size: 1.5rem;
      font-family: var(--font-family);
      outline: none;
      width: 100%;
    }

    .keyboard-popup {
      margin-top: auto;
      margin-bottom: 24px;
      width: 100%;
      max-width: 800px;
      display: flex;
      justify-content: center;
      animation: slideUp 0.4s cubic-bezier(0.2, 0, 0, 1);
    }

    @keyframes fadeIn {
      from {
        opacity: 0;
      }
      to {
        opacity: 1;
      }
    }
    @keyframes slideUp {
      from {
        transform: translateY(100%);
        opacity: 0;
      }
      to {
        transform: translateY(0);
        opacity: 1;
      }
    }
    @keyframes slideDown {
      from {
        transform: translateY(-50%);
        opacity: 0;
      }
      to {
        transform: translateY(0);
        opacity: 1;
      }
    }
  `;

  private handleGlobalClick = (e: MouseEvent | PointerEvent) => {
    if (!this.showKeyboard) return;

    const path = e.composedPath();
    const isInput = path.includes(this.overlayInput);
    const keyboardPopup = this.shadowRoot?.querySelector('.keyboard-popup');
    const isKeyboard = keyboardPopup ? path.includes(keyboardPopup) : false;

    if (!isInput && !isKeyboard) {
      this.closeKeyboard();
    }
  };

  connectedCallback() {
    super.connectedCallback();
    document.addEventListener('pointerdown', this.handleGlobalClick as EventListener);
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    document.removeEventListener('pointerdown', this.handleGlobalClick as EventListener);
  }

  private handleFocus() {
    this.showKeyboard = true;
    // Auto focus the overlay input when it opens
    getAppClock().setTimeout(async () => {
      this.overlayInput?.focus();
    }, 50);
  }

  private closeKeyboard() {
    this.showKeyboard = false;
    this.inputField?.blur();
    this.overlayInput?.blur();
  }

  private handleNativeInput(e: Event) {
    const input = e.target as HTMLInputElement;
    this.inputValue = input.value;
  }

  private handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      this.submitCode();
    }
  }

  private async submitCode() {
    if (this.inputValue.trim()) {
      this.dispatchEvent(
        new CustomEvent('join-meeting-start', {
          detail: { code: this.inputValue.trim() },
          bubbles: true,
          composed: true,
        })
      );
      this.inputValue = '';
      this.showKeyboard = false;
    }
  }

  private handleKeyPress(e: CustomEvent<KeyPressedEvent>) {
    const key = e.detail.key;
    const input = this.overlayInput;
    if (!input) return;

    let start = input.selectionStart || 0;
    let end = input.selectionEnd || 0;
    let val = this.inputValue;

    if (key === 'Dismiss') {
      this.closeKeyboard();
      return;
    } else if (key === 'Enter') {
      this.submitCode();
      return;
    } else if (key === 'Backspace') {
      if (start === end && start > 0) {
        val = val.slice(0, start - 1) + val.slice(end);
        start--;
      } else if (start !== end) {
        val = val.slice(0, start) + val.slice(end);
      }
      end = start;
    } else if (key === 'Left') {
      if (start > 0) start--;
      end = start;
    } else if (key === 'Right') {
      if (start < val.length) start++;
      end = start;
    } else {
      // Insert character
      val = val.slice(0, start) + key + val.slice(end);
      start += key.length;
      end = start;
    }

    this.inputValue = val;
    input.value = val;
    input.setSelectionRange(start, end);
    input.focus();
  }

  render() {
    return html`
      ${
        this.showKeyboard
          ? html`
              <div class="overlay">
                <div class="overlay-input-container">
                  <div class="input-icon">
                    <svg width="32" height="32" viewBox="0 0 24 24" fill="currentColor">
                      <path
                        d="M20 5H4C2.89 5 2.01 5.89 2.01 7L2 17C2 18.11 2.89 19 4 19H20C21.11 19 22 18.11 22 17V7C22 5.89 21.11 5 20 5ZM20 17H4V7H20V17ZM11 8H13V10H11V8ZM11 11H13V13H11V11ZM8 8H10V10H8V8ZM8 11H10V13H8V11ZM5 11H7V13H5V11ZM5 8H7V10H5V8ZM14 11H16V13H14V11ZM14 8H16V10H14V8ZM17 11H19V13H17V11ZM17 8H19V10H17V8ZM8 14H16V16H8V14Z"
                      />
                    </svg>
                  </div>
                  <input
                    type="text"
                    class="overlay-input"
                    placeholder="Enter a code or nickname"
                    .value=${this.inputValue}
                    @input=${this.handleNativeInput}
                    @keydown=${this.handleKeyDown}
                  />
                </div>

                <div class="keyboard-popup">
                  <virtual-keyboard @key-pressed=${this.handleKeyPress}></virtual-keyboard>
                </div>
              </div>
            `
          : ''
      }

      <div class="bottom-bar">
        <div class="bottom-controls">
          <div class="input-container">
            <div class="input-icon">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="currentColor">
                <path
                  d="M20 5H4C2.89 5 2.01 5.89 2.01 7L2 17C2 18.11 2.89 19 4 19H20C21.11 19 22 18.11 22 17V7C22 5.89 21.11 5 20 5ZM20 17H4V7H20V17ZM11 8H13V10H11V8ZM11 11H13V13H11V11ZM8 8H10V10H8V8ZM8 11H10V13H8V11ZM5 11H7V13H5V11ZM5 8H7V10H5V8ZM14 11H16V13H14V11ZM14 8H16V10H14V8ZM17 11H19V13H17V11ZM17 8H19V10H17V8ZM8 14H16V16H8V14Z"
                />
              </svg>
            </div>
            <input
              type="text"
              class="input-field"
              placeholder="Enter a code or nickname"
              .value=${this.inputValue}
              @focus=${this.handleFocus}
              @input=${this.handleNativeInput}
              @keydown=${this.handleKeyDown}
            />
          </div>

          <icon-button>
            <!-- More Icon -->
            <svg viewBox="0 0 24 24">
              <path
                d="M12 8C13.1 8 14 7.1 14 6C14 4.9 13.1 4 12 4C10.9 4 10 4.9 10 6C10 7.1 10.9 8 12 8ZM12 10C10.9 10 10 10.9 10 12C10 13.1 10.9 14 12 14C13.1 14 14 13.1 14 12C14 10.9 13.1 10 12 10ZM12 16C10.9 16 10 16.9 10 18C10 19.1 10.9 20 12 20C13.1 20 14 19.1 14 18C14 16.9 13.1 16 12 16Z"
              />
            </svg>
          </icon-button>
        </div>
      </div>
    `;
  }
}
