import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import Sessions from './Sessions';
import { api } from '../services/api';
import type { Session } from '../types';

vi.mock('../services/api', () => ({
  api: { listSessions: vi.fn(), getClub: vi.fn(), listSessionHistory: vi.fn() },
}));
vi.mock('../context/useAuth', () => ({ useAuth: vi.fn(() => ({ isAdmin: false })) }));

function makeSession(overrides: Partial<Session> = {}): Session {
  return {
    id: 'session-1',
    title: 'Sunday Social',
    description: '',
    session_date: '2026-09-13T00:00:00Z',
    start_time: '18:00',
    end_time: '20:00',
    courts: 2,
    max_players: 10,
    rsvp_deadline: '2026-09-10T23:59:59Z',
    is_recurring: false,
    recurring_day_of_week: null,
    recurring_parent_id: null,
    status: 'open',
    created_by: 'admin-1',
    created_at: '2026-09-01T00:00:00Z',
    updated_at: '2026-09-01T00:00:00Z',
    rsvps: [],
    ...overrides,
  };
}

function renderPage() {
  return render(
    <MemoryRouter>
      <Sessions />
    </MemoryRouter>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.getClub).mockResolvedValue({ venue_name: 'Olympic Park' } as never);
  vi.mocked(api.listSessionHistory).mockResolvedValue({ items: [], total: 0 });
});

describe('Sessions page', () => {
  it('lists the sessions it loads', async () => {
    vi.mocked(api.listSessions).mockResolvedValue([
      makeSession({ id: 's1', title: 'Sunday Social' }),
      makeSession({ id: 's2', title: 'Thursday Drills' }),
    ]);

    renderPage();

    expect(await screen.findByText('Sunday Social')).toBeInTheDocument();
    expect(screen.getByText('Thursday Drills')).toBeInTheDocument();
  });

  it('explains the empty state rather than showing a blank page', async () => {
    vi.mocked(api.listSessions).mockResolvedValue([]);

    renderPage();

    expect(await screen.findByText('No upcoming sessions scheduled')).toBeInTheDocument();
  });

  it('stops loading even when the request fails', async () => {
    // A failed load must not leave the spinner up forever.
    vi.mocked(api.listSessions).mockRejectedValue(new Error('network down'));
    vi.spyOn(console, 'error').mockImplementation(() => {});

    renderPage();

    await waitFor(() =>
      expect(screen.getByText('No upcoming sessions scheduled')).toBeInTheDocument()
    );
  });
});

describe('Sessions history tab', () => {
  it('lists sessions that have been played, with what they cost', async () => {
    const user = userEvent.setup();
    vi.mocked(api.listSessionHistory).mockResolvedValue({
      items: [
        {
          session_id: 'p1',
          title: 'Tuesday Social',
          starts_at: '2026-08-25T20:00:00+10:00',
          ends_at: '2026-08-25T22:00:00+10:00',
          settled: true,
          total_cents: 14550,
          player_count: 6,
        },
      ],
      total: 1,
    });

    renderPage();
    await waitFor(() => expect(screen.getByRole('tab', { name: 'History' })).toBeInTheDocument());
    await user.click(screen.getByRole('tab', { name: 'History' }));

    await waitFor(() => expect(screen.getByText('Tuesday Social')).toBeInTheDocument());
    expect(screen.getByText('$145.50')).toBeInTheDocument();
    expect(screen.getByText('6 players')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'See the split' })).toBeInTheDocument();
  });

  // A finished session nobody has costed is the thing the admin most needs to
  // see, so it is surfaced rather than hidden.
  it('flags a finished session that has not been settled', async () => {
    const user = userEvent.setup();
    vi.mocked(api.listSessionHistory).mockResolvedValue({
      items: [
        {
          session_id: 'p2',
          title: 'Thursday Smash',
          starts_at: '2026-08-27T20:00:00+10:00',
          ends_at: '2026-08-27T22:00:00+10:00',
          settled: false,
          total_cents: 0,
          player_count: 0,
        },
      ],
      total: 1,
    });

    renderPage();
    await waitFor(() => expect(screen.getByRole('tab', { name: 'History' })).toBeInTheDocument());
    await user.click(screen.getByRole('tab', { name: 'History' }));

    await waitFor(() => expect(screen.getByText('Not settled')).toBeInTheDocument());
    expect(screen.getByText('Nobody has costed this yet')).toBeInTheDocument();
  });

  it('does not fetch history until the tab is opened', async () => {
    renderPage();

    await waitFor(() => expect(screen.getByRole('tab', { name: 'Upcoming' })).toBeInTheDocument());
    expect(api.listSessionHistory).not.toHaveBeenCalled();
  });
});
