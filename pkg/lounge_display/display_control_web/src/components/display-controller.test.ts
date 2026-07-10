import { WSClient, wsClient } from '../ws-client.js';
import { Ok } from 'standard-ts-lib/src/index.js';
import './display-controller.js';
import { DisplayController } from './display-controller.js';

describe('DisplayController', () => {
  beforeEach(() => {
    jest.spyOn(wsClient, 'request').mockResolvedValue(Ok([]));
  });

  afterEach(() => {
    jest.restoreAllMocks();
    document.body.innerHTML = '';
  });

  it('renders sidebar correctly', async () => {
    const el = document.createElement('display-controller') as DisplayController;
    document.body.appendChild(el);
    await el.updateComplete;
    
    const sidebar = el.shadowRoot!.querySelector('.sidebar');
    expect(sidebar).toBeDefined();
    
    // Sidebar should not be open by default
    expect(sidebar!.classList.contains('open')).toBe(false);
  });

  it('toggles sidebar on button click', async () => {
    const el = document.createElement('display-controller') as DisplayController;
    document.body.appendChild(el);
    await el.updateComplete;
    
    const sidebar = el.shadowRoot!.querySelector('.sidebar')!;
    const btn = el.shadowRoot!.querySelector('.toggle-btn') as HTMLElement;
    
    expect(sidebar.classList.contains('open')).toBe(false);
    
    // Open
    btn.click();
    await el.updateComplete;
    expect(sidebar.classList.contains('open')).toBe(true);
    
    // Close
    btn.click();
    await el.updateComplete;
    expect(sidebar.classList.contains('open')).toBe(false);
  });

  it('hides sidebar when entering a meeting state via wsClient state updates', async () => {
    const el = document.createElement('display-controller') as DisplayController;
    document.body.appendChild(el);
    await el.updateComplete;
    
    const sidebar = el.shadowRoot!.querySelector('.sidebar')!;
    
    // Open sidebar first
    const btn = el.shadowRoot!.querySelector('.toggle-btn') as HTMLElement;
    btn.click();
    await el.updateComplete;
    expect(sidebar.classList.contains('open')).toBe(true);
    expect(sidebar.classList.contains('hidden')).toBe(false);
    
    (wsClient as any).notifyListeners({ meeting_code: 'abc-defg-hij', current_node: 'In Meeting' });
    
    await el.updateComplete;
    
    // It should now be hidden and closed
    expect(sidebar.classList.contains('hidden')).toBe(true);
    expect(sidebar.classList.contains('open')).toBe(false);
    
    // Now return to landing
    (wsClient as any).notifyListeners({ meeting_code: 'landing', current_node: 'landing' });
    await el.updateComplete;
    
    // It should no longer be hidden
    expect(sidebar.classList.contains('hidden')).toBe(false);
  });
});
