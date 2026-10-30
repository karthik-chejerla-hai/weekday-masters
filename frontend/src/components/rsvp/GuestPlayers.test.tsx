import { expect, it } from 'vitest';
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
