import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import PlayerList from './PlayerList';
import type { RSVP } from '../../types';

describe('PlayerList Component', () => {
  const mockRsvps: RSVP[] = [
    {
      id: '1',
      session_id: 's1',
      user_id: 'u1',
      status: 'in',
      rsvp_timestamp: '2026-03-01T10:00:00Z',
      is_late_rsvp: false,
      added_by_admin: false,
      created_at: '2026-03-01T10:00:00Z',
      updated_at: '2026-03-01T10:00:00Z',
      user: {
        id: 'u1',
        auth0_id: 'auth0|1',
        email: 'alice@example.com',
        name: 'Alice Cooper',
        profile_picture: '',
        phone_number: '',
        role: 'player',
        is_player: true,
        membership_status: 'approved',
        created_at: '',
        updated_at: '',
      },
    },
    {
      id: '2',
      session_id: 's1',
      user_id: 'u2',
      status: 'waitlisted',
      waitlist_position: 1,
      rsvp_timestamp: '2026-03-01T11:00:00Z',
      is_late_rsvp: false,
      added_by_admin: false,
      created_at: '2026-03-01T11:00:00Z',
      updated_at: '2026-03-01T11:00:00Z',
      user: {
        id: 'u2',
        auth0_id: 'auth0|2',
        email: 'bob@example.com',
        name: 'Bob Marley',
        profile_picture: '',
        phone_number: '',
        role: 'player',
        is_player: true,
        membership_status: 'approved',
        created_at: '',
        updated_at: '',
      },
    },
    {
      id: '3',
      session_id: 's1',
      user_id: 'u3',
      status: 'maybe',
      rsvp_timestamp: '2026-03-01T12:00:00Z',
      is_late_rsvp: false,
      added_by_admin: false,
      created_at: '2026-03-01T12:00:00Z',
      updated_at: '2026-03-01T12:00:00Z',
      user: {
        id: 'u3',
        auth0_id: 'auth0|3',
        email: 'charlie@example.com',
        name: 'Charlie Brown',
        profile_picture: '',
        phone_number: '',
        role: 'player',
        is_player: true,
        membership_status: 'approved',
        created_at: '',
        updated_at: '',
      },
    },
  ];

  it('renders confirmed, waitlisted, and maybe sections with counts', () => {
    render(<PlayerList rsvps={mockRsvps} maxPlayers={6} />);

    expect(screen.getByText('Alice Cooper')).toBeInTheDocument();
    expect(screen.getByText('Bob Marley')).toBeInTheDocument();
    expect(screen.getByText('Charlie Brown')).toBeInTheDocument();

    expect(screen.getByText('Waitlist')).toBeInTheDocument();
    expect(screen.getByText('Maybe')).toBeInTheDocument();
  });

  it('renders empty state when there are no confirmed players', () => {
    render(<PlayerList rsvps={[]} maxPlayers={6} />);

    expect(screen.getByText('No confirmed players yet')).toBeInTheDocument();
  });
});
