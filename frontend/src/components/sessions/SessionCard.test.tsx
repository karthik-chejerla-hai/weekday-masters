import { describe, it, expect, afterEach, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import SessionCard from './SessionCard';
import type { Session } from '../../types';

describe('SessionCard Component', () => {
  const baseSession: Session = {
    id: 'session-123',
    title: 'Sunday Social Match',
    description: 'Casual badminton games',
    session_date: '2026-04-12T00:00:00Z',
    start_time: '18:00',
    end_time: '20:00',
    courts: 2,
    max_players: 10,
    rsvp_deadline: '2026-04-09T23:59:59Z',
    is_recurring: false,
    recurring_day_of_week: null,
    recurring_parent_id: null,
    status: 'open',
    created_by: 'admin-1',
    created_at: '2026-04-01T00:00:00Z',
    updated_at: '2026-04-01T00:00:00Z',
    rsvps: [
      {
        id: 'r1',
        session_id: 'session-123',
        user_id: 'u1',
        status: 'in',
        rsvp_timestamp: '2026-04-02T10:00:00Z',
        is_late_rsvp: false,
        added_by_admin: false,
        created_at: '',
        updated_at: '',
        user: {
          id: 'u1',
          auth0_id: 'auth0|1',
          email: 'p1@test.com',
          name: 'Player One',
          nickname: '',
          profile_picture: '',
          phone_number: '',
          role: 'player',
          is_player: true,
          membership_status: 'approved',
          created_at: '',
          updated_at: '',
        },
      },
    ],
  };

  it('opens the session with Tab and Enter through one named link', async () => {
    const user = userEvent.setup();
    render(<MemoryRouter><Routes>
      <Route path="/" element={<SessionCard session={baseSession} />} />
      <Route path="/sessions/:id" element={<h1>Session destination</h1>} />
    </Routes></MemoryRouter>);
    const link = screen.getByRole('link', { name: 'Sunday Social Match, Sunday, 12 April 2026' });
    expect(screen.getAllByRole('link')).toHaveLength(1);
    expect(link).toHaveAttribute('href', '/sessions/session-123');
    await user.tab();
    expect(link).toHaveFocus();
    await user.keyboard('{Enter}');
    expect(screen.getByRole('heading', { name: 'Session destination' })).toBeInTheDocument();
  });

  it('renders session title, times, and venue', () => {
    render(
      <MemoryRouter>
        <SessionCard session={baseSession} venueName="Olympic Badminton Hall" />
      </MemoryRouter>
    );

    expect(screen.getByText('Sunday Social Match')).toBeInTheDocument();
    expect(screen.getByText('18:00 - 20:00')).toBeInTheDocument();
    expect(screen.getByText('Olympic Badminton Hall')).toBeInTheDocument();
  });

  it('shows cancelled badge when session is cancelled', () => {
    const cancelledSession: Session = {
      ...baseSession,
      status: 'cancelled',
      cancellation_reason: 'Court unavailable',
    };

    render(
      <MemoryRouter>
        <SessionCard session={cancelledSession} />
      </MemoryRouter>
    );

    expect(screen.getByText('Cancelled')).toBeInTheDocument();
  });

  it('uses the physical court number while preserving booked capacity', () => {
    render(<MemoryRouter><SessionCard session={baseSession} venueName="BadmintonWorx Norwest" courtNumber={8} timeFormat="12h" /></MemoryRouter>);
    expect(screen.getByText('BadmintonWorx Norwest')).toBeInTheDocument();
    expect(screen.getByText('Court 8')).toBeInTheDocument();
    expect(screen.getByText('6:00 PM - 8:00 PM')).toBeInTheDocument();
    expect(screen.getByText('1 of 10 confirmed')).toBeInTheDocument();
    expect(screen.queryByText('2 courts')).not.toBeInTheDocument();
  });

  it('shows one date for a generated title and retains custom titles', () => {
    render(<MemoryRouter><SessionCard session={{ ...baseSession, title: 'Sunday - 12 Apr 2026' }} /></MemoryRouter>);
    expect(screen.getAllByText('Sunday, 12 April 2026')).toHaveLength(1);
    expect(screen.queryByText('Sunday - 12 Apr 2026')).not.toBeInTheDocument();
  });

  it.each([
    ['in', 'Confirmed'], ['maybe', 'Tentative'], ['out', 'Not attending'], ['waitlisted', 'Waitlisted (#2)'],
  ] as const)('shows the caller response for %s without colour or hover', (status, label) => {
    const rsvp = { ...baseSession.rsvps![0], status, waitlist_position: 2 };
    render(<MemoryRouter><SessionCard session={{ ...baseSession, rsvps: [rsvp] }} currentUserId="u1" /></MemoryRouter>);
    expect(screen.getByText(`Your RSVP: ${label}`)).toBeInTheDocument();
  });

  it('keeps every player response visible on the card', () => {
    const confirmed = baseSession.rsvps![0];
    const absent = { ...confirmed, id: 'r2', user_id: 'u2', status: 'out' as const,
      user: { ...confirmed.user!, id: 'u2', nickname: 'Absent player' } };
    render(<MemoryRouter><SessionCard session={{ ...baseSession, rsvps: [confirmed, absent] }} currentUserId="u3" /></MemoryRouter>);
    expect(screen.getByRole('list', { name: 'Player responses' })).toHaveTextContent('Player');
    expect(screen.getByText('Absent player')).toBeInTheDocument();
    expect(screen.getByRole('listitem', { name: 'Absent player, Not attending' })).toBeInTheDocument();
    expect(screen.getByRole('listitem', { name: 'Player, Confirmed' })).toBeInTheDocument();
    expect(screen.getByText('Your RSVP: Not responded')).toBeInTheDocument();
  });

  it('retains both full/waiting and closed states', () => {
    const confirmed = baseSession.rsvps![0];
    render(<MemoryRouter><SessionCard session={{ ...baseSession, max_players: 1,
      rsvp_deadline: '2000-01-01T00:00:00Z',
      rsvps: [confirmed, { ...confirmed, id: 'r2', user_id: 'u2', status: 'waitlisted' }],
    }} /></MemoryRouter>);
    expect(screen.getByText('Full · 1 waiting')).toBeInTheDocument();
    expect(screen.getByText('RSVP Closed')).toBeInTheDocument();
  });

  it('shows only confirmed players and score actions for the current session', () => {
    const confirmed = baseSession.rsvps![0];
    const others = (['out', 'maybe', 'waitlisted'] as const).map((status, i) => ({
      ...confirmed, id: `other-${i}`, user_id: `other-${i}`, status,
      user: { ...confirmed.user!, id: `other-${i}`, nickname: `Other ${i}` },
    }));
    render(<MemoryRouter><SessionCard session={{ ...baseSession, max_players: 1,
      rsvps: [confirmed, ...others],
    }} recordScore currentUserId="u1" /></MemoryRouter>);
    expect(screen.getByText('1 confirmed player')).toBeInTheDocument();
    expect(screen.getByRole('listitem', { name: 'Player, Confirmed' })).toBeInTheDocument();
    expect(screen.getAllByRole('listitem')).toHaveLength(1);
    expect(screen.queryByText(/Your RSVP:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/RSVP Closed|RSVP open|RSVP by|Full|spots left/)).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Record score' })).toHaveAttribute('href', '/sessions/session-123/games');
    expect(screen.getByRole('link', { name: 'Session details' })).toBeInTheDocument();
  });

  it.each([0, 2])('shows %s confirmed players without RSVP messaging', (count) => {
    const rsvps = Array.from({ length: count }, (_, i) => ({ ...baseSession.rsvps![0], id: `r${i}`, user_id: `u${i}` }));
    render(<MemoryRouter><SessionCard session={{ ...baseSession, rsvps }} recordScore /></MemoryRouter>);
    expect(screen.getByText(`${count} confirmed players`)).toBeInTheDocument();
    expect(screen.queryByText('No RSVPs yet')).not.toBeInTheDocument();
    if (count === 0) expect(screen.getByText('No confirmed players yet')).toBeInTheDocument();
  });

  describe('RSVP deadline', () => {
    afterEach(() => {
      vi.useRealTimers();
    });

    // Pin "now" so the deadline on baseSession (9 Apr 2026) is in the future.
    const freezeBefore = () => {
      vi.useFakeTimers();
      vi.setSystemTime(new Date('2026-04-01T00:00:00Z'));
    };

    it('shows the RSVP deadline while the session is still open', () => {
      freezeBefore();

      render(
        <MemoryRouter>
          <SessionCard session={baseSession} />
        </MemoryRouter>
      );

      expect(screen.getByText(/^RSVP by /)).toBeInTheDocument();
      expect(screen.queryByText('RSVP Closed')).not.toBeInTheDocument();
    });

    it.each([
      ['2027-01-10T07:00:00Z', '12h', 'RSVP by Sun, 10 Jan at 6:00 PM'],
      ['2027-01-10T07:00:00Z', '24h', 'RSVP by Sun, 10 Jan at 18:00'],
      ['2027-07-11T08:00:00Z', '12h', 'RSVP by Sun, 11 Jul at 6:00 PM'],
      ['2027-07-11T08:00:00Z', '24h', 'RSVP by Sun, 11 Jul at 18:00'],
    ] as const)('shows the Sydney deadline time for %s in %s format', (deadline, timeFormat, expected) => {
      freezeBefore();
      render(<MemoryRouter><SessionCard session={{ ...baseSession, rsvp_deadline: deadline }} timeFormat={timeFormat} /></MemoryRouter>);
      expect(screen.getByText(expected)).toBeInTheDocument();
    });

    it('hides the RSVP deadline once it has passed', () => {
      vi.useFakeTimers();
      vi.setSystemTime(new Date('2026-04-11T00:00:00Z'));

      render(
        <MemoryRouter>
          <SessionCard session={baseSession} />
        </MemoryRouter>
      );

      expect(screen.queryByText(/^RSVP by /)).not.toBeInTheDocument();
      expect(screen.getByText('RSVP Closed')).toBeInTheDocument();
    });

    it('hides the RSVP deadline on a cancelled session', () => {
      freezeBefore();

      render(
        <MemoryRouter>
          <SessionCard session={{ ...baseSession, status: 'cancelled' }} />
        </MemoryRouter>
      );

      expect(screen.queryByText(/^RSVP by /)).not.toBeInTheDocument();
    });
  });
});
