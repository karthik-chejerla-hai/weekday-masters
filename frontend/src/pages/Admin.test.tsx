import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import Admin from './Admin';
import { api } from '../services/api';

vi.mock('../services/api', () => ({ api: { getClub: vi.fn(), listJoinRequests: vi.fn(), updateClub: vi.fn() } }));

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.listJoinRequests).mockResolvedValue([]);
  vi.mocked(api.getClub).mockResolvedValue({ name: 'Rally', venue_name: 'BadmintonWorx Norwest', venue_address: 'Local venue', court_number: 8, time_format: '24h', notifications_paused: true } as never);
  vi.mocked(api.updateClub).mockResolvedValue({} as never);
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
    expect(api.updateClub).toHaveBeenCalledWith({ name: 'Rally', venue_name: 'BadmintonWorx Norwest', venue_address: 'Local venue', court_number: 8, time_format: '12h', rsvp_deadline_hours: 72 });
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
