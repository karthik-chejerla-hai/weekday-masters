import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Dashboard from './Dashboard';
import { useAuth } from '../context/useAuth';
import { api } from '../services/api';
import type { Session } from '../types';

vi.mock('../context/useAuth', () => ({ useAuth: vi.fn() }));
vi.mock('../services/api', () => ({
  api: { listSessions: vi.fn(), listCancelledSessions: vi.fn(), listUnsettledSessions: vi.fn(), getClub: vi.fn() },
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
  vi.mocked(api.listUnsettledSessions).mockResolvedValue({items: [],total: 0});
  vi.mocked(api.listCancelledSessions).mockResolvedValue([]);
  vi.mocked(api.getClub).mockResolvedValue({ venue_name: 'Olympic Park' } as never);
});

describe('Dashboard page', () => {
  it('greets the member by first name', async () => {
    renderPage();
    expect(await screen.findByText(/Welcome back, Jane/)).toBeInTheDocument();
  });

  it('shows the next game and only one future session', async () => {
    vi.mocked(api.listSessions).mockResolvedValue([
      makeSession({ id: 's1', title: 'Session One' }),
      makeSession({ id: 's2', title: 'Session Two' }),
      makeSession({ id: 's3', title: 'Session Three' }),
      makeSession({ id: 's4', title: 'Session Four' }),
    ]);

    renderPage();

    expect(await screen.findByText('Session One')).toBeInTheDocument();
    expect(screen.getByText('Session Two')).toBeInTheDocument();
    expect(screen.queryByText('Session Three')).not.toBeInTheDocument();
    expect(screen.queryByText('Session Four')).not.toBeInTheDocument();
    expect(screen.queryByText('Later sessions')).not.toBeInTheDocument();
    const future = screen.getByRole('region', { name: 'Future sessions' });
    expect(within(future).getAllByRole('article')).toHaveLength(1);
    expect(within(future).getByText('Session Two')).toBeInTheDocument();
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
    expect(screen.getByText('No next session is scheduled.')).toBeInTheDocument();
  });

  it('stops loading when the requests fail', async () => {
    vi.mocked(api.listSessions).mockRejectedValue(new Error('offline'));
    vi.spyOn(console, 'error').mockImplementation(() => {});

    renderPage();

    await waitFor(() => expect(screen.getByText(/Welcome back/)).toBeInTheDocument());
  });
});

const outstanding = [
  {session_id: 'older',title: 'Older game',ends_at: '2026-09-01T12:00:00Z',player_count: 4,settled:false,total_cents:0},
  {session_id: 'recent',title: 'Recent game',ends_at: '2026-09-08T12:00:00Z',player_count: 5,settled:false,total_cents:0},
];
it('shows all pending expenses above Next game for admins', async () => {
  vi.mocked(useAuth).mockReturnValue({user:{name:'Admin'},isAdmin:true} as ReturnType<typeof useAuth>);
  vi.mocked(api.listSessions).mockResolvedValue([makeSession()]);
  vi.mocked(api.listUnsettledSessions).mockResolvedValue({items:outstanding,total:2});
  renderPage();
  const region=await screen.findByRole('region',{name:/Expenses to record/});
  expect(within(region).getByText('Older game')).toBeInTheDocument();
  expect(within(region).getAllByRole('link',{name:'Record expense'})).toHaveLength(2);
  expect(within(region).getByText(/4 confirmed players/)).toBeInTheDocument();
  expect(region.compareDocumentPosition(screen.getByRole('region',{name:'Your next game'})) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
});
it('shows members the expense status without a write action', async () => {
  vi.mocked(api.listUnsettledSessions).mockResolvedValue({items:outstanding,total:2});
  renderPage();
  expect(await screen.findAllByText('Expense pending')).toHaveLength(2);
  expect(screen.queryByRole('link',{name:'Record expense'})).not.toBeInTheDocument();
  expect(screen.queryByText('$0.00')).not.toBeInTheDocument();
});


it('shows a future next date when a cancelled game and another game have started', async () => {
  const clock = vi.spyOn(Date, 'now').mockReturnValue(Date.parse('2026-10-05T19:10:00+11:00'));
  try {
    vi.mocked(api.listSessions).mockResolvedValue([
      makeSession({ id: 'ongoing', session_date: '2026-10-05T00:00:00Z', start_time: '19:00', starts_at: '2026-10-05T19:00:00+11:00' }),
      makeSession({ id: 'later', status: 'closed', session_date: '2026-10-12T00:00:00Z', starts_at: '2026-10-12T18:00:00+11:00' }),
    ]);
    vi.mocked(api.listCancelledSessions).mockResolvedValue([
      makeSession({ id: 'cancelled', status: 'cancelled', session_date: '2026-10-05T00:00:00Z', starts_at: '2026-10-05T18:00:00+11:00' }),
    ]);
    renderPage();
    expect(await screen.findByText('Next scheduled session: Monday, 12 October 2026')).toBeInTheDocument();
    expect(screen.getByText('No reason provided.')).toBeInTheDocument();
  } finally {
    clock.mockRestore();
  }
});

const playingSession = makeSession({
  id: 'playing', title: 'Tonight’s session',
  starts_at: '2026-10-13T20:00:00+11:00',
  ends_at: '2026-10-13T22:00:00+11:00',
  start_time: '20:00', end_time: '22:00',
});
const followingSession = makeSession({
  id: 'following', title: 'Next week',
  starts_at: '2026-10-20T20:00:00+11:00',
  ends_at: '2026-10-20T22:00:00+11:00',
});

it.each([
  ['before start', '2026-10-13T19:59:59+11:00', false],
  ['at start', '2026-10-13T20:00:00+11:00', true],
  ['during play', '2026-10-13T21:00:00+11:00', true],
  ['at end', '2026-10-13T22:00:00+11:00', false],
])('uses the scheduled playing window %s', async (_, instant, active) => {
  const clock = vi.spyOn(Date, 'now').mockReturnValue(Date.parse(instant));
  try {
    vi.mocked(api.listSessions).mockResolvedValue([playingSession, followingSession]);
    renderPage();
    await screen.findByText('Next week');
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(active ? 'Session in progress' : 'Ready for your next game?');
    if (active) {
      const card = screen.getByRole('region', { name: 'Current session' });
      const scoreLink = within(card).getByRole('link', { name: 'Record score' });
      expect(scoreLink).toHaveAttribute('href', '/sessions/playing/games');
      expect(scoreLink.closest('article')).not.toBeNull();
      expect(scoreLink.parentElement?.closest('a')).toBeNull();
      expect(within(card).getByRole('link', { name: 'Session details' })).toHaveAttribute('href', '/sessions/playing');
      expect(within(card).getByText('Olympic Park')).toBeInTheDocument();
      expect(within(card).getByText('20:00 - 22:00')).toBeInTheDocument();
    } else {
      expect(screen.queryByRole('link', { name: 'Record score' })).not.toBeInTheDocument();
      if (instant.includes('22:00')) expect(screen.queryByText('Tonight’s session')).not.toBeInTheDocument();
    }
  } finally { clock.mockRestore(); }
});

it('puts the active card before expenses and future sessions even when returned later', async () => {
  const clock = vi.spyOn(Date, 'now').mockReturnValue(Date.parse('2026-10-13T21:00:00+11:00'));
  try {
    vi.mocked(api.listSessions).mockResolvedValue([followingSession, playingSession]);
    vi.mocked(api.listUnsettledSessions).mockResolvedValue({ items: outstanding, total: 2 });
    renderPage();
    const current = await screen.findByRole('region', { name: 'Current session' });
    const expenses = screen.getByRole('region', { name: /Expenses to record/ });
    const future = screen.getByRole('region', { name: 'Future sessions' });
    expect(current.compareDocumentPosition(expenses) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(expenses.compareDocumentPosition(future) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(within(future).getByText('Next week')).toBeInTheDocument();
  } finally { clock.mockRestore(); }
});

it('updates the playing state on focus without reloading the page', async () => {
  const clock = vi.spyOn(Date, 'now').mockReturnValue(Date.parse('2026-10-13T19:59:59+11:00'));
  try {
    vi.mocked(api.listSessions).mockResolvedValue([playingSession, followingSession]);
    renderPage();
    await screen.findByText('Tonight’s session');
    clock.mockReturnValue(Date.parse('2026-10-13T20:00:00+11:00'));
    await act(async () => { window.dispatchEvent(new Event('focus')); });
    expect(screen.getByRole('heading', { level: 1, name: 'Session in progress' })).toBeInTheDocument();
    clock.mockReturnValue(Date.parse('2026-10-13T22:00:00+11:00'));
    await act(async () => { window.dispatchEvent(new Event('focus')); });
    expect(screen.queryByRole('link', { name: 'Record score' })).not.toBeInTheDocument();
    expect(screen.queryByText('Tonight’s session')).not.toBeInTheDocument();
  } finally { clock.mockRestore(); }
});

it('does not offer score entry for a cancelled session within its scheduled window', async () => {
  const clock = vi.spyOn(Date, 'now').mockReturnValue(Date.parse('2026-10-13T21:00:00+11:00'));
  try {
    vi.mocked(api.listSessions).mockResolvedValue([{ ...playingSession, status: 'cancelled' }, followingSession]);
    renderPage();
    await screen.findByText('Next week');
    expect(screen.queryByRole('link', { name: 'Record score' })).not.toBeInTheDocument();
  } finally { clock.mockRestore(); }
});
