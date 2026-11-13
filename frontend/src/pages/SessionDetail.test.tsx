import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import SessionDetail from './SessionDetail';
import { useAuth } from '../context/useAuth';
import { api } from '../services/api';
import type { GuestRSVP, RSVP, RSVPSummary, Session } from '../types';

vi.mock('../context/useAuth', () => ({ useAuth: vi.fn() }));
vi.mock('../services/api', () => ({ api: { getSession: vi.fn(), createRSVP: vi.fn(), getClub: vi.fn(), listMembers: vi.fn(), adminAddRSVP: vi.fn(), deleteGuestRSVP: vi.fn() } }));

const ME = 'user-me';

// Dates either side of "now" so deadline behaviour is not tied to the calendar.
const future = (days: number) => new Date(Date.now() + days * 86_400_000).toISOString();
const past = (days: number) => new Date(Date.now() - days * 86_400_000).toISOString();
const guest = { id: 'guest-1', guest_name: 'Alex Guest', status: 'in', rsvp_timestamp: past(1) } as GuestRSVP;

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
  vi.mocked(api.getClub).mockResolvedValue({ venue_name: 'BadmintonWorx Norwest', court_number: 8, time_format: '12h' } as never);
});

describe('SessionDetail page', () => {
  it('keeps RSVP details available if display settings cannot load', async () => {
    vi.mocked(api.getClub).mockRejectedValue(new Error('settings unavailable'));
    loads(makeSession());
    renderPage();
    expect(await screen.findByText('Sunday Social')).toBeInTheDocument();
    expect(screen.getByText('18:00 - 20:00')).toBeInTheDocument();
    expect(screen.getByText('Your RSVP')).toBeInTheDocument();
  });

  it('shows the session and marks it open for RSVP', async () => {
    loads(makeSession());
    renderPage();

    expect(await screen.findByText('Sunday Social')).toBeInTheDocument();
    expect(screen.getByText('Casual games')).toBeInTheDocument();
    expect(screen.getByText('BadmintonWorx Norwest - Court 8')).toBeInTheDocument();
    expect(screen.getByText('6:00 PM - 8:00 PM')).toBeInTheDocument();
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
    loads(makeSession({ status: 'cancelled', cancellation_reason: 'Court flooded' }));
    renderPage();

    expect(await screen.findByText('Cancelled')).toBeInTheDocument();
    expect(screen.getByText('Court flooded')).toBeInTheDocument();
    expect(screen.queryByText('RSVP Deadline:')).not.toBeInTheDocument();
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

it.each([
  ['upcoming', future(1), 'open', false],
  ['started', past(1), 'open', true],
  ['cancelled', past(1), 'cancelled', false],
] as const)('uses the correct results emphasis for a %s session', async (_, starts_at, status, primary) => {
  loads(makeSession({ starts_at, status }));
  renderPage();
  const link = await screen.findByRole('link', { name: 'Game scores and results' });
  expect(link).toHaveAttribute('href', '/sessions/session-1/games');
  expect(link.classList.contains('btn-primary')).toBe(primary);
});

it.each(['in', 'maybe', 'out', 'waitlisted'] as const)('identifies the current %s response through text and pressed state', async status => {
  loads(makeSession({ rsvps: [makeRSVP({ status })] }));
  renderPage();
  expect(await screen.findByText(/^Current response:/)).toHaveTextContent({
    in: 'Confirmed', maybe: 'Tentative', out: 'Not attending', waitlisted: 'Waitlisted',
  }[status]);
  expect(screen.getByRole('button', { pressed: true })).toBeInTheDocument();
});


describe('admin player RSVPs', () => {
  beforeEach(() => {
    vi.mocked(useAuth).mockReturnValue({ user: { id: ME, role: 'admin' } } as unknown as ReturnType<typeof useAuth>);
    vi.mocked(api.listMembers).mockResolvedValue([
      { id: 'player-1', name: 'Alice Smith', nickname: 'Ace', membership_status: 'approved' },
      { id: 'pending-1', name: 'Pending Person', membership_status: 'pending' },
    ] as never);
    vi.mocked(api.adminAddRSVP).mockResolvedValue({} as never);
  });

  it.each([false, true])('saves a player RSVP with deadline passed = %s and refreshes the roster', async deadlinePassed => {
    const session = makeSession({ rsvp_deadline: deadlinePassed ? past(1) : future(1) });
    loads(session);
    renderPage();
    await userEvent.selectOptions(await screen.findByLabelText('Player'), 'player-1');
    expect(screen.queryByRole('option', { name: 'Pending' })).not.toBeInTheDocument();
    loads({ ...session, rsvps: [makeRSVP({ user_id: 'player-1' })] }, makeSummary({ total_in: 1, spots_left: 9 }));
    await userEvent.click(screen.getByRole('button', { name: 'Save player RSVP' }));
    expect(await screen.findByText("Ace's RSVP saved as In.")).toBeInTheDocument();
    expect(api.adminAddRSVP).toHaveBeenCalledWith('session-1', 'player-1', 'in');
    expect(api.getSession).toHaveBeenCalledTimes(2);
    expect(screen.getByText('Current response: In')).toBeInTheDocument();
    expect(api.createRSVP).not.toHaveBeenCalled();
    if (deadlinePassed) expect(screen.getByText(/deadline stays unchanged/)).toBeInTheDocument();
  });

  it('changes an existing response and explains the capacity override', async () => {
    loads(makeSession({ rsvps: [makeRSVP({ user_id: 'player-1', status: 'maybe' })] }), makeSummary({ spots_left: 0, total_in: 10 }));
    renderPage();
    await userEvent.selectOptions(await screen.findByLabelText('Player'), 'player-1');
    expect(screen.getByLabelText('Response')).toHaveValue('maybe');
    await userEvent.selectOptions(screen.getByLabelText('Response'), 'in');
    expect(screen.getByText(/confirms the player beyond capacity/)).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText('Response'), 'out');
    await userEvent.click(screen.getByRole('button', { name: 'Save player RSVP' }));
    expect(await screen.findByText("Ace's RSVP saved as Out.")).toBeInTheDocument();
    expect(api.adminAddRSVP).toHaveBeenCalledWith('session-1', 'player-1', 'out');
  });

  it('shows a failure and allows retry without changing the roster', async () => {
    loads(makeSession());
    vi.mocked(api.adminAddRSVP).mockRejectedValueOnce(new Error('offline'));
    renderPage();
    await userEvent.selectOptions(await screen.findByLabelText('Player'), 'player-1');
    await userEvent.click(screen.getByRole('button', { name: 'Save player RSVP' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not save this RSVP');
    expect(api.getSession).toHaveBeenCalledTimes(1);
    await userEvent.click(screen.getByRole('button', { name: 'Save player RSVP' }));
    expect(await screen.findByText("Ace's RSVP saved as In.")).toBeInTheDocument();
  });

  it.each(['closed', 'cancelled'] as const)('hides admin controls on a %s session', async status => {
    loads(makeSession({ status }));
    renderPage();
    await screen.findByText('Sunday Social');
    expect(screen.queryByLabelText('Player')).not.toBeInTheDocument();
    expect(api.listMembers).not.toHaveBeenCalled();
  });

  it('keeps admin controls hidden from ordinary members', async () => {
    vi.mocked(useAuth).mockReturnValue({ user: { id: ME, role: 'player' } } as unknown as ReturnType<typeof useAuth>);
    loads(makeSession());
    renderPage();
    await screen.findByText('Sunday Social');
    expect(screen.queryByLabelText('Player')).not.toBeInTheDocument();
    expect(api.listMembers).not.toHaveBeenCalled();
  });

  it('removes a guest from the player row and reloads the roster without opening the editor', async () => {
    loads(makeSession({ guest_rsvps: [guest] }));
    vi.mocked(api.deleteGuestRSVP).mockImplementationOnce(async () => { loads(makeSession()); });
    renderPage();
    await userEvent.click(await screen.findByRole('button', { name: "Remove Alex Guest's RSVP" }));
    expect(api.deleteGuestRSVP).toHaveBeenCalledExactlyOnceWith('session-1', guest.id);
    expect(await screen.findByText('No confirmed players yet')).toBeInTheDocument();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it.each([
    ['member', 'open', future(1)],
    ['admin', 'closed', future(1)],
    ['admin', 'cancelled', future(1)],
    ['admin', 'open', past(1)],
  ] as const)('hides guest removal for %s on a %s session starting %s', async (role, status, starts_at) => {
    vi.mocked(useAuth).mockReturnValue({ user: { id: ME, role } } as unknown as ReturnType<typeof useAuth>);
    loads(makeSession({ guest_rsvps: [guest], status, starts_at }));
    renderPage();
    await screen.findByText('Alex Guest');
    expect(screen.queryByRole('button', { name: "Remove Alex Guest's RSVP" })).not.toBeInTheDocument();
  });
});
