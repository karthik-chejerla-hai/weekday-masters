import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import SessionDetail from './SessionDetail';
import { useAuth } from '../context/useAuth';
import { api } from '../services/api';
import type { RSVP, RSVPSummary, Session } from '../types';

vi.mock('../context/useAuth', () => ({ useAuth: vi.fn() }));
vi.mock('../services/api', () => ({ api: { getSession: vi.fn(), createRSVP: vi.fn() } }));

const ME = 'user-me';

// Dates either side of "now" so deadline behaviour is not tied to the calendar.
const future = (days: number) => new Date(Date.now() + days * 86_400_000).toISOString();
const past = (days: number) => new Date(Date.now() - days * 86_400_000).toISOString();

function makeSession(overrides: Partial<Session> = {}): Session {
  return {
    id: 'session-1',
    title: 'Sunday Social',
    description: 'Casual games',
    session_date: future(10),
    start_time: '18:00',
    end_time: '20:00',
    courts: 2,
    max_players: 10,
    rsvp_deadline: future(7),
    is_recurring: false,
    recurring_day_of_week: null,
    recurring_parent_id: null,
    status: 'open',
    created_by: 'admin-1',
    created_at: past(30),
    updated_at: past(30),
    rsvps: [],
    ...overrides,
  };
}

function makeSummary(overrides: Partial<RSVPSummary> = {}): RSVPSummary {
  return {
    total_in: 0,
    total_out: 0,
    total_maybe: 0,
    total_waitlisted: 0,
    max_players: 10,
    spots_left: 10,
    ...overrides,
  } as RSVPSummary;
}

function makeRSVP(overrides: Partial<RSVP> = {}): RSVP {
  return {
    id: 'rsvp-1',
    session_id: 'session-1',
    user_id: ME,
    status: 'in',
    rsvp_timestamp: past(1),
    is_late_rsvp: false,
    added_by_admin: false,
    created_at: past(1),
    updated_at: past(1),
    ...overrides,
  } as RSVP;
}

function loads(session: Session, summary = makeSummary()) {
  vi.mocked(api.getSession).mockResolvedValue({ session, rsvp_summary: summary } as never);
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/sessions/session-1']}>
      <Routes>
        <Route path="/sessions/:id" element={<SessionDetail />} />
      </Routes>
    </MemoryRouter>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(useAuth).mockReturnValue({ user: { id: ME } } as unknown as ReturnType<typeof useAuth>);
  vi.mocked(api.createRSVP).mockResolvedValue({} as never);
});

describe('SessionDetail page', () => {
  it('shows the session and marks it open for RSVP', async () => {
    loads(makeSession());
    renderPage();

    expect(await screen.findByText('Sunday Social')).toBeInTheDocument();
    expect(screen.getByText('Casual games')).toBeInTheDocument();
    expect(screen.getByText('Open for RSVP')).toBeInTheDocument();
  });

  it('reports a session that could not be loaded', async () => {
    vi.mocked(api.getSession).mockRejectedValue(new Error('not found'));
    vi.spyOn(console, 'error').mockImplementation(() => {});

    renderPage();

    expect(await screen.findByText('Session not found')).toBeInTheDocument();
  });

  it('submits an RSVP and reloads the session', async () => {
    loads(makeSession());
    renderPage();

    await userEvent.click(await screen.findByRole('button', { name: /i'm in/i }));

    expect(api.createRSVP).toHaveBeenCalledWith('session-1', 'in');
    // Reloading is what refreshes the roster and the waitlist positions.
    expect(api.getSession).toHaveBeenCalledTimes(2);
  });

  it('closes RSVP once the deadline has passed', async () => {
    loads(makeSession({ rsvp_deadline: past(1) }));
    renderPage();

    expect(await screen.findByText('RSVP Closed')).toBeInTheDocument();
    expect(screen.getByText('RSVP is closed for this session')).toBeInTheDocument();
  });

  it('locks a confirmed player in after the deadline', async () => {
    // The rule the backend enforces: past the deadline you cannot drop out.
    loads(makeSession({
      rsvp_deadline: past(1),
      rsvps: [makeRSVP({ status: 'in' })],
    }));
    renderPage();

    expect(await screen.findByText('RSVP deadline has passed')).toBeInTheDocument();
    expect(screen.getByText(/cannot change your RSVP/i)).toBeInTheDocument();
  });

  it('marks a cancelled session and hides the RSVP controls', async () => {
    loads(makeSession({ status: 'cancelled' }));
    renderPage();

    expect(await screen.findByText('Cancelled')).toBeInTheDocument();
    expect(screen.queryByText('Your RSVP')).not.toBeInTheDocument();
  });

  it('explains the waitlist position to a waitlisted player', async () => {
    loads(
      makeSession({ rsvps: [makeRSVP({ status: 'waitlisted', waitlist_position: 3 })] }),
      makeSummary({ total_in: 10, spots_left: 0, total_waitlisted: 1 })
    );
    renderPage();

    expect(await screen.findByText(/You're on the waitlist at position #3/)).toBeInTheDocument();
  });

  it('warns that a full session will put the member on the waitlist', async () => {
    loads(makeSession(), makeSummary({ total_in: 10, spots_left: 0 }));
    renderPage();

    expect(await screen.findByText('This session is full')).toBeInTheDocument();
    expect(screen.getByText(/join the waitlist/i)).toBeInTheDocument();
  });
});
