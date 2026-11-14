import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import SessionCard from './SessionCard';
import { api } from '../../services/api';
import type { Session } from '../../types';
vi.mock('../../services/api', () => ({ api: { previewRSVPReminder: vi.fn(), sendRSVPReminder: vi.fn() } }));
const session = {
  id: 'session-1', title: 'Sunday badminton', session_date: '2099-04-12T00:00:00Z',
  start_time: '18:00', end_time: '20:00', starts_at: '2099-04-12T08:00:00Z',
  max_players: 10, rsvp_deadline: '2099-04-10T08:00:00Z', status: 'open',
} as Session;
const batch = {
  id: 'batch-1', request_id: 'request-1', session_id: session.id, audience_count: 3,
  queued_count: 2, suppressed_count: 1, next_allowed_at: '2099-01-01T00:00:00Z', created_at: '',
};
function card(overrides: Partial<Session> = {}, isAdmin = true) {
  return render(<MemoryRouter><SessionCard session={{ ...session, ...overrides }} isAdmin={isAdmin} /></MemoryRouter>);
}
beforeEach(() => {
  vi.mocked(api.previewRSVPReminder).mockResolvedValue({ audience_count: 3, next_allowed_at: null });
  vi.mocked(api.sendRSVPReminder).mockResolvedValue(batch);
});
afterEach(() => { vi.clearAllMocks(); vi.useRealTimers(); });
describe('Session RSVP reminders', () => {
  it('requires confirmation and reports queued and suppressed counts', async () => {
    const user = userEvent.setup(); card();
    await user.click(screen.getByRole('button', { name: 'Remind players to RSVP' }));
    expect(await screen.findByText('Remind 3 members to RSVP for Sunday badminton?')).toBeInTheDocument();
    expect(api.sendRSVPReminder).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', { name: 'Confirm reminders' }));
    expect(await screen.findByRole('status')).toHaveTextContent('2 push reminders queued. 1 could not be queued');
    expect(api.sendRSVPReminder).toHaveBeenCalledWith(session.id, expect.any(String));
    expect(screen.getByRole('button', { name: 'Remind players to RSVP' })).toBeDisabled();
  });
  it('allows cancelling without sending', async () => {
    const user = userEvent.setup(); card();
    await user.click(screen.getByRole('button', { name: 'Remind players to RSVP' }));
    await user.click(await screen.findByRole('button', { name: 'Cancel' }));
    expect(api.sendRSVPReminder).not.toHaveBeenCalled();
    expect(screen.queryByRole('region', { name: 'Confirm RSVP reminders' })).not.toBeInTheDocument();
  });
  it('hides the action for non-admins', () => {
    card({}, false);
    expect(screen.queryByRole('button', { name: 'Remind players to RSVP' })).not.toBeInTheDocument();
  });
  it.each([
    { status: 'cancelled' as const }, { status: 'closed' as const },
    { rsvp_deadline: '2000-01-01T00:00:00Z' }, { starts_at: '2000-01-01T00:00:00Z' },
  ])('disables reminders for ineligible sessions: %o', overrides => {
    card(overrides);
    expect(screen.getByRole('button', { name: 'Remind players to RSVP' })).toBeDisabled();
    expect(api.previewRSVPReminder).not.toHaveBeenCalled();
  });
  it('shows a shared cooldown discovered during preview', async () => {
    vi.mocked(api.previewRSVPReminder).mockResolvedValue({ audience_count: 3, next_allowed_at: batch.next_allowed_at });
    const user = userEvent.setup(); card();
    await user.click(screen.getByRole('button', { name: 'Remind players to RSVP' }));
    expect(await screen.findByRole('status')).toHaveTextContent('last hour');
    expect(screen.queryByRole('button', { name: 'Confirm reminders' })).not.toBeInTheDocument();
  });
  it('does not offer confirmation when everyone has responded', async () => {
    vi.mocked(api.previewRSVPReminder).mockResolvedValue({ audience_count: 0, next_allowed_at: null });
    const user = userEvent.setup(); card();
    await user.click(screen.getByRole('button', { name: 'Remind players to RSVP' }));
    expect(await screen.findByRole('status')).toHaveTextContent('Everyone has already responded');
    expect(api.sendRSVPReminder).not.toHaveBeenCalled();
  });
  it('retries an uncertain result with the same request ID', async () => {
    vi.mocked(api.sendRSVPReminder).mockRejectedValueOnce(new Error('network'));
    const user = userEvent.setup(); card();
    await user.click(screen.getByRole('button', { name: 'Remind players to RSVP' }));
    await user.click(await screen.findByRole('button', { name: 'Confirm reminders' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Try again');
    await user.click(screen.getByRole('button', { name: 'Retry confirmation' }));
    await waitFor(() => expect(api.sendRSVPReminder).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.sendRSVPReminder).mock.calls[0]).toEqual(vi.mocked(api.sendRSVPReminder).mock.calls[1]);
  });
});


it.each(['deadline', 'start', 'cancelled'] as const)('recovers an uncertain request after %s closes new sends', async change => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date('2026-11-14T00:00:00Z'));
  vi.mocked(api.sendRSVPReminder).mockRejectedValueOnce(new Error('response lost'));
  const nearDeadline = { ...session, starts_at: '2026-11-14T01:00:00Z', rsvp_deadline: '2026-11-14T00:00:02Z' };
  const view = card(nearDeadline);
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Remind players to RSVP' })); });
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Confirm reminders' })); });
  expect(screen.getByRole('alert')).toHaveTextContent('Try again');
  const firstRequest = vi.mocked(api.sendRSVPReminder).mock.calls[0];
  await act(async () => { vi.advanceTimersByTime(change === 'start' ? 3_601_000 : 3_000); });
  if (change === 'cancelled') {
    view.rerender(<MemoryRouter><SessionCard session={{ ...nearDeadline, status: 'cancelled' }} isAdmin /></MemoryRouter>);
  }
  expect(screen.getByRole('button', { name: 'Remind players to RSVP' })).toBeDisabled();
  expect(screen.getByRole('button', { name: 'Retry confirmation' })).toBeEnabled();
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Retry confirmation' })); });
  expect(vi.mocked(api.sendRSVPReminder).mock.calls[1]).toEqual(firstRequest);
  expect(screen.getByRole('status')).toHaveTextContent('2 push reminders queued');
});

it('does not allow a first submission after a pending confirmation expires', async () => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date('2026-11-14T00:00:00Z'));
  card({ starts_at: '2026-11-14T01:00:00Z', rsvp_deadline: '2026-11-14T00:00:02Z' });
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Remind players to RSVP' })); });
  await act(async () => { vi.advanceTimersByTime(3_000); });
  expect(screen.getByRole('button', { name: 'Confirm reminders' })).toBeDisabled();
  expect(api.sendRSVPReminder).not.toHaveBeenCalled();
});


it('honours a definitive cooldown rejection instead of offering recovery', async () => {
  vi.mocked(api.sendRSVPReminder).mockRejectedValueOnce({ response: { status: 429, data: {
    message: 'This session was reminded in the last hour.', next_allowed_at: batch.next_allowed_at,
  } } });
  const user = userEvent.setup(); card();
  await user.click(screen.getByRole('button', { name: 'Remind players to RSVP' }));
  await user.click(await screen.findByRole('button', { name: 'Confirm reminders' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('last hour');
  expect(screen.getByRole('button', { name: 'Confirm reminders' })).toBeDisabled();
  expect(screen.queryByRole('button', { name: 'Retry confirmation' })).not.toBeInTheDocument();
});
