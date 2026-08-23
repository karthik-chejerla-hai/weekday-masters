import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Dashboard from './Dashboard';
import { useAuth } from '../context/useAuth';
import { api } from '../services/api';
import type { Session } from '../types';

vi.mock('../context/useAuth', () => ({ useAuth: vi.fn() }));
vi.mock('../services/api', () => ({
  api: { listSessions: vi.fn(), listCancelledSessions: vi.fn(), getClub: vi.fn() },
}));

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
      <Dashboard />
    </MemoryRouter>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(useAuth).mockReturnValue({
    user: { name: 'Jane Player' },
  } as unknown as ReturnType<typeof useAuth>);
  vi.mocked(api.listSessions).mockResolvedValue([]);
  vi.mocked(api.listCancelledSessions).mockResolvedValue([]);
  vi.mocked(api.getClub).mockResolvedValue({ venue_name: 'Olympic Park' } as never);
});

describe('Dashboard page', () => {
  it('greets the member by first name', async () => {
    renderPage();
    expect(await screen.findByText(/Welcome back, Jane/)).toBeInTheDocument();
  });

  it('shows at most three upcoming sessions', async () => {
    vi.mocked(api.listSessions).mockResolvedValue([
      makeSession({ id: 's1', title: 'Session One' }),
      makeSession({ id: 's2', title: 'Session Two' }),
      makeSession({ id: 's3', title: 'Session Three' }),
      makeSession({ id: 's4', title: 'Session Four' }),
    ]);

    renderPage();

    expect(await screen.findByText('Session One')).toBeInTheDocument();
    expect(screen.getByText('Session Three')).toBeInTheDocument();
    // The dashboard is a preview; the fourth belongs on the sessions page.
    expect(screen.queryByText('Session Four')).not.toBeInTheDocument();
  });

  it('surfaces cancelled sessions so members are not left waiting', async () => {
    vi.mocked(api.listCancelledSessions).mockResolvedValue([
      makeSession({
        id: 'c1',
        status: 'cancelled',
        cancellation_reason: 'Court flooded',
      }),
    ]);

    renderPage();

    // The banner identifies the session by date, and carries the reason.
    expect(await screen.findByText(/Session Cancelled:/)).toBeInTheDocument();
    expect(screen.getByText('Court flooded')).toBeInTheDocument();
  });

  it('stops loading when the requests fail', async () => {
    vi.mocked(api.listSessions).mockRejectedValue(new Error('offline'));
    vi.spyOn(console, 'error').mockImplementation(() => {});

    renderPage();

    await waitFor(() => expect(screen.getByText(/Welcome back/)).toBeInTheDocument());
  });
});
