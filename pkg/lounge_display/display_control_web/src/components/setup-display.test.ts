import './setup-display.js';
import { SetupDisplay } from './setup-display.js';
import { wsClient } from '../ws-client.js';
import { getAppClock, setAppClock } from '../clock-provider.js';
import { FakeClock } from 'standard-ts-lib/src/clock.js';

// Mock QRCode
jest.mock('qrcode', () => ({
  toDataURL: jest.fn().mockResolvedValue('data:image/png;base64,mockqr')
}));

describe('SetupDisplay', () => {
  let clock: FakeClock;
  let requestSpy: jest.SpyInstance;
  let alertSpy: jest.SpyInstance;

  beforeEach(() => {
    clock = new FakeClock(0);
    setAppClock(clock);
    requestSpy = jest.spyOn(wsClient, 'request').mockResolvedValue({});
    alertSpy = jest.spyOn(window, 'alert').mockImplementation(() => {});
  });

  afterEach(() => {
    document.body.innerHTML = '';
    jest.clearAllMocks();
  });

  it('renders correctly and shows checklist after timeout', async () => {
    const el = document.createElement('setup-display') as SetupDisplay;
    document.body.appendChild(el);
    await el.updateComplete;
    
    expect((el as any).showChecklist).toBe(false);
    
    clock.addMillis(1500);
    await clock.executeTimeoutFuncs();
    await el.updateComplete;
    
    expect((el as any).showChecklist).toBe(true);
    
    // Cleanup
    el.remove();
  });

  it('handles Init Server node and wifi check', async () => {
    requestSpy.mockImplementation((req: any) => {
      if (req.type === 'has_wifi') return Promise.resolve({ internetAccess: true });
      return Promise.resolve({});
    });
    
    const el = document.createElement('setup-display') as SetupDisplay;
    document.body.appendChild(el);
    await el.updateComplete;
    
    (wsClient as any).notifyListeners({ current_node: 'Init Server', phase: 'setup', setup_phase: 1 });
    
    // Allow promises to resolve
    await new Promise(r => setTimeout(r, 0));
    await el.updateComplete;
    
    expect(requestSpy).toHaveBeenCalledWith({ type: 'has_wifi' });
    expect((el as any).statusPageText).toBe('Server Initializing...');
  });

  it('handles Init Server node without wifi', async () => {
    requestSpy.mockImplementation((req: any) => {
      if (req.type === 'has_wifi') return Promise.resolve({ internetAccess: false });
      return Promise.resolve({});
    });
    
    const el = document.createElement('setup-display') as SetupDisplay;
    document.body.appendChild(el);
    await el.updateComplete;
    
    (wsClient as any).notifyListeners({ current_node: 'Init Server', phase: 'setup', setup_phase: 1 });
    
    // Allow promises to resolve
    await new Promise(r => setTimeout(r, 0));
    await el.updateComplete;
    
    expect((el as any).statusPageText).toBe('Please Connect this Pi to the internet');
  });

  it('handles Credentials Phase', async () => {
    requestSpy.mockImplementation((req: any) => {
      if (req.type === 'get_ip') return Promise.resolve({ ip: '192.168.1.100' });
      return Promise.resolve({});
    });
    
    const el = document.createElement('setup-display') as SetupDisplay;
    document.body.appendChild(el);
    await el.updateComplete;
    
    (wsClient as any).notifyListeners({ current_node: 'Credentials Phase', phase: 'setup', setup_phase: 2 });
    
    await new Promise(r => setTimeout(r, 0));
    await el.updateComplete;
    
    expect(requestSpy).toHaveBeenCalledWith({ type: 'get_ip' });
    expect((el as any).extraHtml).toBeDefined();
  });

  it('handles Auth Token Phase and token submission', async () => {
    requestSpy.mockImplementation((req: any) => {
      if (req.type === 'get_auth_url') return Promise.resolve({ url: 'https://auth.example.com' });
      if (req.type === 'submit_token') return Promise.resolve({ status: 'ok' });
      return Promise.resolve({});
    });
    
    const el = document.createElement('setup-display') as SetupDisplay;
    document.body.appendChild(el);
    await el.updateComplete;
    
    (wsClient as any).notifyListeners({ current_node: 'Auth Token Phase', phase: 'setup', setup_phase: 3 });
    
    await new Promise(r => setTimeout(r, 0));
    await el.updateComplete;
    
    expect(requestSpy).toHaveBeenCalledWith({ type: 'get_auth_url' });
    
    // Submit token
    const tokenInput = el.shadowRoot!.querySelector('#token-input') as any;
    tokenInput.value = 'my-token';
    const submitBtn = el.shadowRoot!.querySelector('md-filled-button') as HTMLElement;
    submitBtn.click();
    
    await new Promise(r => setTimeout(r, 0));
    await el.updateComplete;
    
    expect(requestSpy).toHaveBeenCalledWith({ type: 'submit_token', payload: { code: 'my-token' } });
    expect((el as any).statusPageText).toBe('Token submitted, processing...');
  });

  it('handles Password Input Page and submission', async () => {
    requestSpy.mockImplementation((req: any) => {
      if (req.type === 'submit_password') return Promise.resolve({ status: 'ok' });
      return Promise.resolve({});
    });
    
    const el = document.createElement('setup-display') as SetupDisplay;
    document.body.appendChild(el);
    await el.updateComplete;
    
    (wsClient as any).notifyListeners({ current_node: 'Password Input Page', phase: 'setup', setup_phase: 13 });
    
    await new Promise(r => setTimeout(r, 0));
    await el.updateComplete;
    
    const passwordInput = el.shadowRoot!.querySelector('#google-password-input') as any;
    passwordInput.value = 'secret';
    const submitBtn = el.shadowRoot!.querySelector('md-filled-button') as HTMLElement;
    submitBtn.click();
    
    await new Promise(r => setTimeout(r, 0));
    await el.updateComplete;
    
    expect(requestSpy).toHaveBeenCalledWith({ type: 'submit_password', payload: { password: 'secret' } });
  });
  
  it('handles failed token and password submission', async () => {
    requestSpy.mockImplementation((req: any) => {
      if (req.type === 'get_auth_url') return Promise.resolve({ url: 'https://auth.example.com' });
      if (req.type === 'submit_token') return Promise.reject(new Error('fail'));
      if (req.type === 'submit_password') return Promise.reject(new Error('fail'));
      return Promise.resolve({});
    });
    
    const el = document.createElement('setup-display') as SetupDisplay;
    document.body.appendChild(el);
    await el.updateComplete;
    
    // Token
    (wsClient as any).notifyListeners({ current_node: 'Auth Token Phase', phase: 'setup', setup_phase: 3 });
    await new Promise(r => setTimeout(r, 0));
    await el.updateComplete;
    
    const tokenInput = el.shadowRoot!.querySelector('#token-input') as any;
    tokenInput.value = 'bad-token';
    (el.shadowRoot!.querySelector('md-filled-button') as HTMLElement).click();
    
    await new Promise(r => setTimeout(r, 0));
    expect(alertSpy).toHaveBeenCalledWith('Failed to submit token');
    
    // Password
    (wsClient as any).notifyListeners({ current_node: 'Password Input Page', phase: 'setup', setup_phase: 13 });
    await new Promise(r => setTimeout(r, 0));
    await el.updateComplete;
    
    const passwordInput = el.shadowRoot!.querySelector('#google-password-input') as any;
    passwordInput.value = 'bad-password';
    (el.shadowRoot!.querySelector('md-filled-button') as HTMLElement).click();
    
    await new Promise(r => setTimeout(r, 0));
    expect(alertSpy).toHaveBeenCalledWith('Failed to submit password');
  });

  it('triggers setup phase 1000 countdown and reloads', async () => {
    const el = document.createElement('setup-display') as SetupDisplay;
    const reloadSpy = jest.spyOn(SetupDisplay.prototype as any, 'reloadPage').mockImplementation(() => {});
    document.body.appendChild(el);
    await el.updateComplete;
    
    (wsClient as any).notifyListeners({ setup_phase: 1000 });
    await el.updateComplete;
    
    expect((el as any).allClear).toBe(true);
    
    // Fast forward countdown (15 seconds)
    for (let i = 0; i < 15; i++) {
      clock.addMillis(1000);
      await clock.executeTimeoutFuncs();
    }
    
    expect(reloadSpy).toHaveBeenCalled();
  });

  it('handles password input logic', async () => {
    const el = document.createElement('setup-display') as SetupDisplay;
    document.body.appendChild(el);
    await el.updateComplete;
    
    (wsClient as any).notifyListeners({ current_node: 'Password Input Page', phase: 'setup', setup_phase: 4 });
    await el.updateComplete;
    
    const pwInput = el.shadowRoot!.querySelector('#google-password-input') as HTMLElement;
    pwInput.dispatchEvent(new Event('focus'));
    
    await el.updateComplete;
    expect((el as any).activeKeyboardInput).toBe('password');
  });

  it('handles virtual keyboard logic', async () => {
    requestSpy.mockImplementation((req: any) => {
      if (req.type === 'get_auth_url') return Promise.resolve({ url: 'https://auth.example.com' });
      if (req.type === 'submit_token') return Promise.resolve({ status: 'ok' });
      return Promise.resolve({});
    });
    
    const el = document.createElement('setup-display') as SetupDisplay;
    document.body.appendChild(el);
    await el.updateComplete;
    
    (wsClient as any).notifyListeners({ current_node: 'Auth Token Phase', phase: 'setup', setup_phase: 3 });
    await new Promise(r => setTimeout(r, 0));
    await el.updateComplete;
    
    const tokenInput = el.shadowRoot!.querySelector('#token-input') as any;
    
    // Focus to open keyboard
    tokenInput.dispatchEvent(new Event('focus'));
    await el.updateComplete;
    
    expect((el as any).activeKeyboardInput).toBe('token');
    
    // Simulate key press
    (el as any).handleKeyPress(new CustomEvent('key-pressed', { detail: { key: 'a' } }));
    expect((el as any).keyboardInputValue).toBe('a');
    
    // Backspace
    (el as any).handleKeyPress(new CustomEvent('key-pressed', { detail: { key: 'Backspace' } }));
    expect((el as any).keyboardInputValue).toBe('');
    
    // Left / Right (ignored)
    (el as any).handleKeyPress(new CustomEvent('key-pressed', { detail: { key: 'Left' } }));
    expect((el as any).keyboardInputValue).toBe('');
    
    // Simulate Enter (submits)
    (el as any).keyboardInputValue = 'token-from-kb';
    tokenInput.value = 'token-from-kb'; // mimic data sync
    (el as any).handleKeyPress(new CustomEvent('key-pressed', { detail: { key: 'Enter' } }));
    
    await new Promise(r => setTimeout(r, 0));
    expect(requestSpy).toHaveBeenCalledWith({ type: 'submit_token', payload: { code: 'token-from-kb' } });
    
    // Test input typing in overlay
    tokenInput.dispatchEvent(new Event('focus'));
    await el.updateComplete;
    
    const overlayInput = el.shadowRoot!.querySelector('.overlay-input') as HTMLInputElement;
    overlayInput.value = 'typed-manually';
    overlayInput.dispatchEvent(new Event('input', { bubbles: true, composed: true }));
    expect((el as any).keyboardInputValue).toBe('typed-manually');
    expect(tokenInput.value).toBe('typed-manually');
    
    // Test visibility toggle
    const toggleBtn = el.shadowRoot!.querySelector('.overlay-input-container > div') as HTMLElement;
    toggleBtn.click();
    await el.updateComplete;
    expect((el as any).showKeyboardInputText).toBe(true);
    toggleBtn.click();
    await el.updateComplete;
    expect((el as any).showKeyboardInputText).toBe(false);
    
    // Test Dismiss
    (el as any).handleKeyPress(new CustomEvent('key-pressed', { detail: { key: 'Dismiss' } }));
    expect((el as any).activeKeyboardInput).toBeNull();
    
    // Reactivate for outside click test
    tokenInput.dispatchEvent(new Event('focus'));
    await el.updateComplete;
    
    // Test clicking outside (pointerdown on overlay)
    const overlay = el.shadowRoot!.querySelector('.keyboard-overlay') as HTMLElement;
    overlay.dispatchEvent(new Event('pointerdown', { bubbles: true, composed: true }));
    await el.updateComplete;
    
    expect((el as any).activeKeyboardInput).toBeNull();
  });
});
