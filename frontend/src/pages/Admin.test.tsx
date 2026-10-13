import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import Admin from './Admin';
import { api } from '../services/api';

vi.mock('../context/useAuth', () => ({ useAuth: () => ({ user: { id: 'admin' } }) }));
vi.mock('../services/api', () => ({ api: { getClub: vi.fn(), listJoinRequests: vi.fn(), updateClub: vi.fn(), sendAnnouncement: vi.fn() } }));

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.listJoinRequests).mockResolvedValue([]);
  vi.mocked(api.getClub).mockResolvedValue({ name: 'Rally', venue_name: 'BadmintonWorx Norwest', venue_address: 'Local venue', court_number: 8, time_format: '24h', notifications_paused: true } as never);
  vi.mocked(api.updateClub).mockResolvedValue({} as never);
  vi.mocked(api.sendAnnouncement).mockResolvedValue({ audience_count: 3, queued_count: 1, suppressed_count: 2 } as never);
});

function renderPage() {
  return render(<MemoryRouter><Admin /></MemoryRouter>);
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
