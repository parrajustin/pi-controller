import './meeting-entry.js';
import { MeetingEntry } from './meeting-entry.js';

describe('MeetingEntry', () => {
  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('renders meeting details correctly', async () => {
    const meeting = {
      name: 'Design Sync',
      time: '2:00 PM',
      lengthInSeconds: 3600,
      isActive: false,
      status: '',
      meetCode: 'xyz-abcd-efg'
    };
    
    const el = document.createElement('meeting-entry') as MeetingEntry;
    el.meeting = meeting;
    document.body.appendChild(el);
    await el.updateComplete;
    
    const title = el.shadowRoot!.querySelector('.meeting-name');
    expect(title!.textContent).toBe('Design Sync');
    
    const time = el.shadowRoot!.querySelector('.meeting-time');
    expect(time!.textContent).toContain('2:00 PM');
  });

  it('renders active status correctly', async () => {
    const meeting = {
      name: 'Active Sync',
      time: '2:00 PM',
      lengthInSeconds: 3600,
      isActive: true,
      status: 'Now',
      meetCode: 'xyz'
    };
    
    const el = document.createElement('meeting-entry') as MeetingEntry;
    el.meeting = meeting;
    document.body.appendChild(el);
    await el.updateComplete;
    
    const row = el.shadowRoot!.querySelector('.meeting-row');
    expect(row!.classList.contains('active')).toBe(true);
    
    const status = el.shadowRoot!.querySelector('.meeting-status');
    expect(status!.textContent).toBe('Now');
  });

  it('dispatches join-meeting-start when clicked', async () => {
    const meeting = {
      name: 'Test',
      time: '2:00 PM',
      lengthInSeconds: 3600,
      isActive: false,
      status: '',
      meetCode: 'test-code'
    };
    
    const el = document.createElement('meeting-entry') as MeetingEntry;
    el.meeting = meeting;
    document.body.appendChild(el);
    await el.updateComplete;
    
    let dispatchedCode = '';
    el.addEventListener('join-meeting-start', (e: any) => {
      dispatchedCode = e.detail.code;
    });
    
    const row = el.shadowRoot!.querySelector('.meeting-row') as HTMLElement;
    row.click();
    
    expect(dispatchedCode).toBe('test-code');
  });

  it('does not dispatch if meetCode is missing', async () => {
    const meeting = {
      name: 'Test',
      time: '2:00 PM',
      lengthInSeconds: 3600,
      isActive: false,
      status: '',
      meetCode: undefined
    };
    
    const el = document.createElement('meeting-entry') as MeetingEntry;
    el.meeting = meeting;
    document.body.appendChild(el);
    await el.updateComplete;
    
    let dispatched = false;
    el.addEventListener('join-meeting-start', () => {
      dispatched = true;
    });
    
    const row = el.shadowRoot!.querySelector('.meeting-row') as HTMLElement;
    row.click();
    
    expect(dispatched).toBe(false);
  });

  it('does not dispatch if isLoading is true', async () => {
    const meeting = {
      name: 'Test',
      time: '2:00 PM',
      lengthInSeconds: 3600,
      isActive: false,
      status: '',
      meetCode: 'test-code'
    };
    
    const el = document.createElement('meeting-entry') as MeetingEntry;
    el.meeting = meeting;
    el.isLoading = true;
    document.body.appendChild(el);
    await el.updateComplete;
    
    let dispatched = false;
    el.addEventListener('join-meeting-start', () => {
      dispatched = true;
    });
    
    const row = el.shadowRoot!.querySelector('.meeting-row') as HTMLElement;
    row.click();
    
    expect(dispatched).toBe(false);
  });
});
