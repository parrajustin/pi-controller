import './virtual-keyboard.js';
import { VirtualKeyboard } from './virtual-keyboard.js';

describe('VirtualKeyboard', () => {
  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('renders all rows of keys', async () => {
    const el = document.createElement('virtual-keyboard') as VirtualKeyboard;
    document.body.appendChild(el);
    await el.updateComplete;
    
    const rows = el.shadowRoot!.querySelectorAll('.row');
    expect(rows.length).toBe(4);
    
    const keys = el.shadowRoot!.querySelectorAll('.key');
    expect(keys.length).toBeGreaterThan(30);
  });

  it('dispatches key-pressed events with correct detail', async () => {
    const el = document.createElement('virtual-keyboard') as VirtualKeyboard;
    document.body.appendChild(el);
    await el.updateComplete;
    
    const keys = Array.from(el.shadowRoot!.querySelectorAll('.key')) as HTMLElement[];
    
    let dispatchedKey = '';
    el.addEventListener('key-pressed', (e: any) => {
      dispatchedKey = e.detail.key;
    });
    
    // Find the 'q' key
    const qKey = keys.find(k => k.textContent?.trim() === 'q');
    qKey!.click();
    expect(dispatchedKey).toBe('q');
    
    // Find Backspace (⌫)
    const bsKey = keys.find(k => k.textContent?.trim() === '⌫');
    bsKey!.click();
    expect(dispatchedKey).toBe('Backspace');
    
    // Find Enter (↵)
    const enterKey = keys.find(k => k.textContent?.trim() === '↵');
    enterKey!.click();
    expect(dispatchedKey).toBe('Enter');
    
    // Find Left (←)
    const leftKey = keys.find(k => k.textContent?.trim() === '←');
    leftKey!.click();
    expect(dispatchedKey).toBe('Left');
    
    // Find Right (→)
    const rightKey = keys.find(k => k.textContent?.trim() === '→');
    rightKey!.click();
    expect(dispatchedKey).toBe('Right');
    
    // Find Space (empty text content but has space class)
    const spaceKey = keys.find(k => k.classList.contains('space'));
    spaceKey!.click();
    expect(dispatchedKey).toBe(' ');
    
    // Find Dismiss (svg icon inside)
    const dismissKey = keys.find(k => k.querySelector('svg'));
    dismissKey!.click();
    expect(dispatchedKey).toBe('Dismiss');
  });
});
