import { describe, it, expect, afterEach, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
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
