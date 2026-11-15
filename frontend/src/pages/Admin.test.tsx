import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import Admin from './Admin';
import { api } from '../services/api';

vi.mock('../context/useAuth', () => ({ useAuth: () => ({ user: { id: 'admin' } }) }));
vi.mock('../services/api', () => ({ api: { getClub: vi.fn(), listJoinRequests: vi.fn(), updateClub: vi.fn(), sendAnnouncement: vi.fn(), approveJoinRequest: vi.fn(), rejectJoinRequest: vi.fn() } }));

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.listJoinRequests).mockResolvedValue([]);
  vi.mocked(api.getClub).mockResolvedValue({ name: 'Rally', venue_name: 'BadmintonWorx Norwest', venue_address: 'Local venue', court_number: 8, time_format: '24h', notifications_paused: true } as never);
  vi.mocked(api.updateClub).mockResolvedValue({} as never);
  vi.mocked(api.sendAnnouncement).mockResolvedValue({ audience_count: 3, queued_count: 1, suppressed_count: 2 } as never);
});

function renderPage(openPanels = true) {
  const result = render(<MemoryRouter><Admin /></MemoryRouter>);
  if (openPanels) {
    fireEvent.click(screen.getByText('Edit club settings'));
    fireEvent.click(screen.getByText('Write an announcement'));
  }
  return result;
}

describe('Club display settings', () => {
  it('loads saved settings and sends the selected display format and numeric court', async () => {
    const user = userEvent.setup();
    renderPage();
    await waitFor(() => expect(screen.getByLabelText('Court number')).toHaveValue(8));
    expect(screen.getByLabelText('Time display')).toHaveValue('24h');
    await user.selectOptions(screen.getByLabelText('Time display'), '12h');
    await user.click(screen.getByRole('button', { name: 'Save Club Settings' }));
    expect(await screen.findByText('Club settings saved!')).toBeInTheDocument();
    expect(api.updateClub).toHaveBeenCalledWith({ name: 'Rally', venue_name: 'BadmintonWorx Norwest', venue_address: 'Local venue', court_number: 8, time_format: '12h' });
  });

  it('can clear the assigned court without changing the time choice', async () => {
    const user = userEvent.setup();
    renderPage();
    await waitFor(() => expect(screen.getByLabelText('Court number')).toHaveValue(8));
    await user.clear(screen.getByLabelText('Court number'));
    await user.click(screen.getByRole('button', { name: 'Save Club Settings' }));
    expect(await screen.findByText('Club settings saved!')).toBeInTheDocument();
    expect(api.updateClub).toHaveBeenCalledWith(expect.objectContaining({ court_number: 0, time_format: '24h' }));
  });

  it('refuses a fractional court number before saving', async () => {
    const user = userEvent.setup();
    renderPage();
    await waitFor(() => expect(screen.getByLabelText('Court number')).toHaveValue(8));
    await user.clear(screen.getByLabelText('Court number'));
    await user.type(screen.getByLabelText('Court number'), '8.5');
    await user.click(screen.getByRole('button', { name: 'Save Club Settings' }));
    expect(await screen.findByText(/Enter a whole court number/)).toBeInTheDocument();
    expect(api.updateClub).not.toHaveBeenCalled();
  });
});

describe('Announcement publication',()=>{
 it('retains the request ID after an uncertain response and shows queued/suppressed counts',async()=>{
  const user=userEvent.setup();vi.mocked(api.sendAnnouncement).mockRejectedValueOnce(new Error('response lost'));
  renderPage();await screen.findByText(/Push delivery is paused/);
  await user.type(screen.getByLabelText('Title'),'Court closure');await user.type(screen.getByLabelText('Message'),'Friday courts are closed.');
  const publish=screen.getByRole('button',{name:'Publish announcement'});expect(publish).toBeEnabled();
  await user.click(publish);expect(await screen.findByRole('alert')).toHaveTextContent('Retry without changing');
  const request=vi.mocked(api.sendAnnouncement).mock.calls[0][2];expect(request).toMatch(/^[0-9a-f-]{36}$/);
  await user.click(publish);expect(await screen.findByText('Announcement saved for 3 members. Push queued for 1; 2 will not receive push.')).toBeInTheDocument();
  expect(api.sendAnnouncement).toHaveBeenNthCalledWith(2,'Court closure','Friday courts are closed.',request);
  expect(screen.queryByText(/sent to all members/)).not.toBeInTheDocument();expect(screen.getByLabelText('Title')).toHaveValue('');
 });
 it('edits after a failed response generate a new request identity instead of conflicting reuse',async()=>{
  const user=userEvent.setup();vi.mocked(api.sendAnnouncement).mockRejectedValueOnce(new Error('offline'));
  renderPage();await user.type(screen.getByLabelText('Title'),'Court closure');await user.type(screen.getByLabelText('Message'),'Message');
  await user.click(screen.getByRole('button',{name:'Publish announcement'}));await screen.findByRole('alert');
  const first=vi.mocked(api.sendAnnouncement).mock.calls[0][2];await user.type(screen.getByLabelText('Message'),' updated');
  await user.click(screen.getByRole('button',{name:'Publish announcement'}));await screen.findByText(/Announcement saved/);
  expect(vi.mocked(api.sendAnnouncement).mock.calls[1][2]).not.toBe(first);
 });
 it('shows conflict feedback without claiming publication success',async()=>{
  const user=userEvent.setup();vi.mocked(api.sendAnnouncement).mockRejectedValueOnce({response:{data:{message:'Request ID already used with different content.'}}});
  renderPage();await user.type(screen.getByLabelText('Title'),'News');await user.type(screen.getByLabelText('Message'),'Message');
  await user.click(screen.getByRole('button',{name:'Publish announcement'}));expect(await screen.findByRole('alert')).toHaveTextContent('Request ID already used');expect(screen.queryByText(/Announcement saved/)).not.toBeInTheDocument();
 });
});

describe('Default RSVP deadline', () => {
 it('loads and saves hours and explains the effect on future sessions', async () => {
  vi.mocked(api.getClub).mockResolvedValue({ name: 'Rally', rsvp_deadline_hours: 48 } as never);
  const user = userEvent.setup();
  renderPage();
  const input = await screen.findByLabelText('Default RSVP deadline (hours before start)');
  await waitFor(() => expect(input).toHaveValue(48));
  expect(screen.getByText(/including custom deadlines/i)).toBeInTheDocument();
  await user.clear(input);
  await user.type(input, '24');
  await user.click(screen.getByRole('button', { name: 'Save Club Settings' }));
  await waitFor(() => expect(api.updateClub).toHaveBeenCalledWith(expect.objectContaining({ rsvp_deadline_hours: 24 })));
 });
 it.each(['', '-1', '1.5', '8761'])('rejects invalid default %j', async (value) => {
  const user = userEvent.setup();
  renderPage();
  const input = await screen.findByLabelText('Default RSVP deadline (hours before start)');
  await user.clear(input);
  if (value) await user.type(input, value);
  await user.click(screen.getByRole('button', { name: 'Save Club Settings' }));
  expect(await screen.findByText(/Enter whole hours from 0 to 8760/)).toBeInTheDocument();
  expect(api.updateClub).not.toHaveBeenCalled();
 });
});

describe('Deadline changes in club settings', () => {
  it('does not send the old default when an admin changes another setting', async () => {
    vi.mocked(api.getClub).mockResolvedValue({ name: 'Rally', rsvp_deadline_hours: 50 } as never);
    // Another tab changed the saved default to 48 after this page loaded.
    vi.mocked(api.updateClub).mockResolvedValue({ rsvp_deadline_hours: 48 } as never);
    const user = userEvent.setup();
    renderPage();
    const input = await screen.findByLabelText('Default RSVP deadline (hours before start)');
    await waitFor(() => expect(input).toHaveValue(50));
    await user.click(screen.getByRole('button', { name: 'Save Club Settings' }));
    await waitFor(() => expect(api.updateClub).toHaveBeenCalled());
    expect(vi.mocked(api.updateClub).mock.calls[0][0]).not.toHaveProperty('rsvp_deadline_hours');
    await waitFor(() => expect(input).toHaveValue(48));
  });

  it('does not send the same deadline again after saving it', async () => {
    vi.mocked(api.updateClub).mockResolvedValue({ rsvp_deadline_hours: 48 } as never);
    const user = userEvent.setup();
    renderPage();
    const input = await screen.findByLabelText('Default RSVP deadline (hours before start)');
    await waitFor(() => expect(input).toHaveValue(50));
    await user.clear(input);
    await user.type(input, '48');
    const save = screen.getByRole('button', { name: 'Save Club Settings' });
    await user.click(save);
    await screen.findByText('Club settings saved!');
    await user.click(save);
    await waitFor(() => expect(api.updateClub).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.updateClub).mock.calls[0][0]).toHaveProperty('rsvp_deadline_hours', 48);
    expect(vi.mocked(api.updateClub).mock.calls[1][0]).not.toHaveProperty('rsvp_deadline_hours');
  });
});


describe('Admin work priorities', () => {
  it('starts with compact pending status and collapsed forms', async () => {
    const user = userEvent.setup();
    const { container } = renderPage(false);
    expect(await screen.findByText('No pending join requests')).toBeVisible();
    expect(screen.getByText('Save Club Settings')).not.toBeVisible();
    expect(screen.getByLabelText('Title')).not.toBeVisible();
    const headings = [...container.querySelectorAll('h2')].map(el => el.textContent);
    expect(headings[0]).toContain('Join Requests');
    expect(screen.getByRole('link', { name: 'Manage sessions' })).toHaveAttribute('href', '/admin/sessions');
    await user.click(screen.getByText('Edit club settings'));
    expect(screen.getByRole('button', { name: 'Save Club Settings' })).toBeVisible();
    await user.click(screen.getByText('Write an announcement'));
    expect(screen.getByRole('textbox', { name: 'Title' })).toBeVisible();
  });
  it('keeps approval and rejection available before routine controls', async () => {
    vi.mocked(api.listJoinRequests).mockResolvedValue([
      { id: 'alice', name: 'Alice Test', email: 'alice@example.invalid' },
      { id: 'bob', name: 'Bob Test', email: 'bob@example.invalid' },
    ] as never);
    const user = userEvent.setup();
    renderPage(false);
    await user.click(await screen.findByRole('button', { name: 'Approve Alice' }));
    await waitFor(() => expect(api.approveJoinRequest).toHaveBeenCalledWith('alice'));
    expect(screen.queryByRole('button', { name: 'Approve Alice' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Reject Bob' }));
    await waitFor(() => expect(api.rejectJoinRequest).toHaveBeenCalledWith('bob'));
    expect(await screen.findByText('No pending join requests')).toBeVisible();
  });
});

describe('Feedback after collapsing pending actions', () => {
  it.each([false, true])('keeps a late club save result visible (failure=%s)', async (failure) => {
    let finish!: () => void;
    vi.mocked(api.updateClub).mockImplementation(() => new Promise((resolve, reject) => {
      finish = () => failure ? reject(new Error('offline')) : resolve({} as never);
    }));
    const user = userEvent.setup();
    renderPage();
    await waitFor(() => expect(screen.getByLabelText('Court number')).toHaveValue(8));
    await user.click(screen.getByRole('button', { name: 'Save Club Settings' }));
    await user.click(screen.getByText('Edit club settings'));
    expect(screen.getByText('Saving club settings…')).toBeVisible();
    finish();
    const message = await screen.findByText(failure ? 'Failed to save club settings' : 'Club settings saved!');
    expect(message).toBeVisible();
  });

  it.each([false, true])('keeps a late announcement result visible (failure=%s)', async (failure) => {
    let finish!: () => void;
    vi.mocked(api.sendAnnouncement).mockImplementation(() => new Promise((resolve, reject) => {
      finish = () => failure ? reject(new Error('response lost')) : resolve({ audience_count: 3, queued_count: 1, suppressed_count: 2 } as never);
    }));
    const user = userEvent.setup();
    renderPage();
    await user.type(screen.getByLabelText('Title'), 'News');
    await user.type(screen.getByLabelText('Message'), 'Club news');
    await user.click(screen.getByRole('button', { name: 'Publish announcement' }));
    await user.click(screen.getByText('Write an announcement'));
    expect(screen.getByText('Publishing announcement…')).toBeVisible();
    finish();
    const message = await screen.findByText(failure ? /Retry without changing/ : /Announcement saved for 3 members/);
    expect(message).toBeVisible();
    expect(message).toHaveTextContent(failure ? 'Retry without changing' : 'Announcement saved for 3 members');
  });
});
