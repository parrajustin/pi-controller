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

});
