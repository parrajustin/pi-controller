import { wsClient } from '../ws-client.js';
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

  it('opens restart options and triggers reboot api when Restart Display is clicked', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({})
    }) as any;

    const el = document.createElement('display-controller') as DisplayController;
    document.body.appendChild(el);
    await el.updateComplete;
    
    // Open sidebar
    const btn = el.shadowRoot!.querySelector('.toggle-btn') as HTMLElement;
    btn.click();
    await el.updateComplete;

    // Click Refresh button
    const refreshBtn = Array.from(el.shadowRoot!.querySelectorAll('.nav-item'))
      .find(n => n.textContent?.includes('Refresh')) as HTMLElement;
    expect(refreshBtn).toBeDefined();
    
    refreshBtn.click();
    await el.updateComplete;

    // Verify dialog appears
    const dialog = el.shadowRoot!.querySelector('.restart-dialog');
    expect(dialog).toBeDefined();

    // Click Restart Display
    const restartBtn = Array.from(dialog!.querySelectorAll('md-filled-button'))
      .find(b => b.textContent?.includes('Restart Display')) as HTMLElement;
    expect(restartBtn).toBeDefined();

    restartBtn.click();
    await el.updateComplete;

    // Verify fetch was called
    expect(global.fetch).toHaveBeenCalledWith('/api/reboot/', expect.objectContaining({
      method: 'POST'
    }));
  });

});
