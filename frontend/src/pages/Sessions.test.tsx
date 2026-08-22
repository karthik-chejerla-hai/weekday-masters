import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Sessions from './Sessions';
import { api } from '../services/api';
import type { Session } from '../types';

vi.mock('../services/api', () => ({
  api: { listSessions: vi.fn(), getClub: vi.fn() },
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
      <Sessions />
    </MemoryRouter>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.getClub).mockResolvedValue({ venue_name: 'Olympic Park' } as never);
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
