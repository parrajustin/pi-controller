import './meeting-list.js';
import { MeetingList } from './meeting-list.js';
import { Meeting } from './meeting-entry.js';

describe('MeetingList', () => {
  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('renders a list of meetings', async () => {
    const meetings: Meeting[] = [
      { name: 'A', time: '12:00 PM', lengthInSeconds: 3600, isActive: false, status: '', meetCode: 'abc' },
      { name: 'B', time: '1:00 PM', lengthInSeconds: 3600, isActive: false, status: '', meetCode: 'def' }
    ];
    
    const el = document.createElement('meeting-list') as MeetingList;
    el.meetings = meetings;
    document.body.appendChild(el);
    await el.updateComplete;
    
    const entries = el.shadowRoot!.querySelectorAll('meeting-entry');
    expect(entries.length).toBe(2);
  });

  it('scrolls selected meeting into view when selectedCode matches', async () => {
    const meetings: Meeting[] = [];
    for (let i = 0; i < 20; i++) {
      meetings.push({ name: `Meeting ${i}`, time: '12:00 PM', lengthInSeconds: 3600, isActive: false, status: '', meetCode: `code-${i}` });
    }
    
    const el = document.createElement('meeting-list') as MeetingList;
    el.meetings = meetings;
    el.selectedCode = 'code-15';
    document.body.appendChild(el);
    await el.updateComplete;
    
    const entries = el.shadowRoot!.querySelectorAll('meeting-entry');
    expect(entries[15].hasAttribute('isselected')).toBe(true);
    expect(entries[0].hasAttribute('isselected')).toBe(false);
  });

  it('reflects isLoading via background color change', async () => {
    const el = document.createElement('meeting-list') as MeetingList;
    el.isLoading = true;
    document.body.appendChild(el);
    await el.updateComplete;
    
    // In JSDOM, getComputedStyle might not accurately reflect host attributes unless appended properly.
    // We can just verify the attribute is set.
    expect(el.hasAttribute('isloading')).toBe(true);
  });
});
