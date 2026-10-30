import { expect, it, vi } from 'vitest';
import userEvent from '@testing-library/user-event';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import PlayerList from './PlayerList';
import SessionCard from '../sessions/SessionCard';
import type { GuestRSVP, Session } from '../../types';
const guest = { id: 'g1', guest_name: 'Alex', charged_to_user_id: 'p1', charged_to_user: { id: 'p1', name: 'Pat Smith' }, status: 'in', rsvp_timestamp: '2026-10-31T00:00:00Z' } as GuestRSVP;
it('shows the guest name and payer in the player list', () => {
  render(<PlayerList rsvps={[guest]} maxPlayers={6} />);
  expect(screen.getByText('Alex')).toBeInTheDocument();
  expect(screen.getByText('Guest')).toBeInTheDocument();
  expect(screen.getByText('Charged to Pat')).toBeInTheDocument();
  expect(screen.getByText('1 / 6')).toBeInTheDocument();
});
it('includes the guest in capacity without setting the payer RSVP', () => {
  const session = { id: 's1', title: 'Sunday', session_date: '2099-11-01', start_time: '18:00', end_time: '20:00', rsvp_deadline: '2099-10-30T00:00:00Z', status: 'open', max_players: 1, guest_rsvps: [guest], rsvps: [] } as unknown as Session;
  render(<MemoryRouter><SessionCard session={session} currentUserId="p1" /></MemoryRouter>);
  expect(screen.getByText('1 of 1 confirmed')).toBeInTheDocument();
  expect(screen.getByText('Full')).toBeInTheDocument();
  expect(screen.getByLabelText('Alex (Guest), Confirmed')).toBeInTheDocument();
  expect(screen.getByText('Your RSVP: Not responded')).toBeInTheDocument();
});

it('offers editing for both IN and OUT guests only when enabled', async () => {
  const onEdit = vi.fn();
  const user = userEvent.setup();
  const { rerender } = render(<PlayerList rsvps={[guest]} maxPlayers={6} onEditGuest={onEdit} />);
  await user.click(screen.getByRole('button', { name: "Edit Alex's RSVP" }));
  expect(onEdit).toHaveBeenLastCalledWith(guest);
  const out = { ...guest, status: 'out' as const };
  rerender(<PlayerList rsvps={[out]} maxPlayers={6} onEditGuest={onEdit} />);
  await user.click(screen.getByRole('button', { name: "Edit Alex's RSVP" }));
  expect(onEdit).toHaveBeenLastCalledWith(out);
  rerender(<PlayerList rsvps={[out]} maxPlayers={6} />);
  expect(screen.queryByRole('button', { name: "Edit Alex's RSVP" })).not.toBeInTheDocument();
});

it.each(['in', 'out'] as const)('removes an %s guest directly from the row', async status => {
  const onRemove = vi.fn().mockResolvedValue(undefined);
  const onEdit = vi.fn();
  const response = { ...guest, status };
  const { rerender } = render(<PlayerList rsvps={[response]} maxPlayers={6} onEditGuest={onEdit} onRemoveGuest={onRemove} />);
  await userEvent.click(screen.getByRole('button', { name: "Remove Alex's RSVP" }));
  expect(onRemove).toHaveBeenCalledExactlyOnceWith(response);
  expect(onEdit).not.toHaveBeenCalled();
  rerender(<PlayerList rsvps={[response]} maxPlayers={6} />);
  expect(screen.queryByRole('button', { name: "Remove Alex's RSVP" })).not.toBeInTheDocument();
});

it('disables guest actions during removal and allows retry after a failure', async () => {
  let rejectRemoval!: (error: Error) => void;
  const onRemove = vi.fn().mockImplementationOnce(() => new Promise<void>((_, reject) => { rejectRemoval = reject; }))
    .mockResolvedValue(undefined);
  render(<PlayerList rsvps={[guest]} maxPlayers={6} onEditGuest={vi.fn()} onRemoveGuest={onRemove} />);
  await userEvent.click(screen.getByRole('button', { name: "Remove Alex's RSVP" }));
  expect(screen.getByRole('button', { name: "Remove Alex's RSVP" })).toBeDisabled();
  expect(screen.getByRole('button', { name: "Edit Alex's RSVP" })).toBeDisabled();
  rejectRemoval(new Error('offline'));
  expect(await screen.findByRole('alert')).toHaveTextContent('Could not remove Alex');
  expect(screen.getByText('Alex')).toBeInTheDocument();
  await userEvent.click(screen.getByRole('button', { name: "Remove Alex's RSVP" }));
  expect(onRemove).toHaveBeenCalledTimes(2);
  expect(screen.queryByRole('alert')).not.toBeInTheDocument();
});
