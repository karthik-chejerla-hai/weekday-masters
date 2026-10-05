import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import AdminSessions from './AdminSessions';
import { api } from '../services/api';
import type { Session } from '../types';

vi.mock('../services/api', () => ({
  api: {
    getClub: vi.fn(),
    listSessions: vi.fn(),
    listCancelledSessions: vi.fn(),
    createSession: vi.fn(),
    deleteSession: vi.fn(),
    cancelSession: vi.fn(),
  },
}));

function makeSession(overrides: Partial<Session> = {}): Session {
  return {
    id: 'session-1',
    title: 'Sunday Social',
    description: '',
    session_date: '2026-09-13T00:00:00Z',
    start_time: '20:00',
    end_time: '22:00',
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
      <AdminSessions />
    </MemoryRouter>
  );
}

/** Opens the create form and returns the form element. */
async function openForm(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: /new session/i }));
  return screen.getByRole('button', { name: /create session/i }).closest('form') as HTMLFormElement;
}

function deadlineToggle() {
  return screen.getByRole('checkbox', { name: /custom rsvp deadline/i });
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.listSessions).mockResolvedValue([]);
  vi.mocked(api.listCancelledSessions).mockResolvedValue([]);
  vi.mocked(api.getClub).mockResolvedValue({ time_format: '24h' } as never);
  vi.mocked(api.createSession).mockResolvedValue(makeSession());
});

describe('AdminSessions page', () => {
  it('keeps the schedule usable if the display settings cannot load', async () => {
    vi.mocked(api.getClub).mockRejectedValue(new Error('settings unavailable'));
    vi.mocked(api.listSessions).mockResolvedValue([makeSession()]);
    renderPage();
    expect(await screen.findByText('Sunday Social')).toBeInTheDocument();
    expect(screen.getByText(/20:00 - 22:00/)).toBeInTheDocument();
  });

  it('keeps all scheduled games visible and uses the club time format', async () => {
    vi.mocked(api.getClub).mockResolvedValue({ time_format: '12h' } as never);
    vi.mocked(api.listSessions).mockResolvedValue([1, 2, 3, 4].map(n => makeSession({ id: `s${n}`, title: `Game ${n}` })));
    renderPage();
    expect(await screen.findByText('Game 4')).toBeInTheDocument();
    expect(screen.getAllByText(/8:00 PM - 10:00 PM/)).toHaveLength(4);
  });

  it('lists the sessions it loads', async () => {
    vi.mocked(api.listSessions).mockResolvedValue([
      makeSession({ id: 's1', title: 'Sunday Social' }),
      makeSession({ id: 's2', title: 'Thursday Drills' }),
    ]);

    renderPage();

    expect(await screen.findByText('Sunday Social')).toBeInTheDocument();
    expect(screen.getByText('Thursday Drills')).toBeInTheDocument();
  });

  it('invites the admin to create one when there are none', async () => {
    renderPage();

    expect(await screen.findByText(/click "new session" to create one/i)).toBeInTheDocument();
  });

  describe('RSVP deadline', () => {
    it('sends no deadline when the admin does not customise it', async () => {
      const user = userEvent.setup();
      renderPage();
      await screen.findByText(/click "new session"/i);

      const form = await openForm(user);
      await user.type(form.querySelector('input[type="date"]')!, '2026-12-11');
      await user.click(screen.getByRole('button', { name: /create session/i }));

      await waitFor(() => expect(api.createSession).toHaveBeenCalled());
      expect(vi.mocked(api.createSession).mock.calls[0][0].rsvp_deadline).toBeUndefined();
    });

    it('offers a datetime picker for a one-off session', async () => {
      const user = userEvent.setup();
      renderPage();
      await screen.findByText(/click "new session"/i);

      const form = await openForm(user);
      expect(form.querySelector('input[type="datetime-local"]')).toBeNull();

      await user.click(deadlineToggle());

      expect(form.querySelector('input[type="datetime-local"]')).not.toBeNull();
      expect(screen.queryByText(/days before session/i)).not.toBeInTheDocument();
    });

    it('offers a relative offset for a recurring session', async () => {
      const user = userEvent.setup();
      renderPage();
      await screen.findByText(/click "new session"/i);

      const form = await openForm(user);
      await user.click(screen.getByRole('radio', { name: /recurring weekly/i }));
      await user.click(deadlineToggle());

      expect(screen.getByText(/days before session/i)).toBeInTheDocument();
      expect(screen.getByText(/deadline time/i)).toBeInTheDocument();
      expect(form.querySelector('input[type="datetime-local"]')).toBeNull();
    });

    it('sends a one-off deadline as an RFC3339 timestamp', async () => {
      const user = userEvent.setup();
      renderPage();
      await screen.findByText(/click "new session"/i);

      const form = await openForm(user);
      await user.type(form.querySelector('input[type="date"]')!, '2026-12-11');
      await user.click(deadlineToggle());
      await user.type(form.querySelector('input[type="datetime-local"]')!, '2026-12-08T18:00');
      await user.click(screen.getByRole('button', { name: /create session/i }));

      await waitFor(() => expect(api.createSession).toHaveBeenCalled());

      const sent = vi.mocked(api.createSession).mock.calls[0][0].rsvp_deadline;
      expect(sent).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{2}:\d{2}$/);
      // Whatever the runner's zone, it must denote the instant the admin picked.
      expect(new Date(sent!).getTime()).toBe(new Date('2026-12-08T18:00').getTime());
    });

    it('derives a recurring deadline from the days-before offset', async () => {
      const user = userEvent.setup();
      renderPage();
      await screen.findByText(/click "new session"/i);

      const form = await openForm(user);
      await user.click(screen.getByRole('radio', { name: /recurring weekly/i }));
      await user.type(form.querySelector('input[type="date"]')!, '2026-12-11');
      await user.click(deadlineToggle());

      // Labels here are not bound to their inputs, so find the offset select
      // by an option only it carries.
      const offsetSelect = Array.from(form.querySelectorAll('select')).find((el) =>
        within(el).queryByText('2 days before')
      ) as HTMLSelectElement;
      expect(offsetSelect).toBeTruthy();

      await user.selectOptions(offsetSelect, '2');
      await user.click(screen.getByRole('button', { name: /create session/i }));

      await waitFor(() => expect(api.createSession).toHaveBeenCalled());

      const sent = vi.mocked(api.createSession).mock.calls[0][0].rsvp_deadline;
      expect(sent).toBeDefined();
      // 2 days before the 11th, at the default 23:59.
      expect(new Date(sent!).getTime()).toBe(
        new Date('2026-12-09T23:59:59').getTime()
      );
    });

    it('refuses a deadline in the past without calling the API', async () => {
      const user = userEvent.setup();
      renderPage();
      await screen.findByText(/click "new session"/i);

      const form = await openForm(user);
      await user.type(form.querySelector('input[type="date"]')!, '2026-12-11');
      await user.click(deadlineToggle());
      await user.type(form.querySelector('input[type="datetime-local"]')!, '2020-01-01T18:00');
      await user.click(screen.getByRole('button', { name: /create session/i }));

      expect(await screen.findByText(/rsvp deadline cannot be in the past/i)).toBeInTheDocument();
      expect(api.createSession).not.toHaveBeenCalled();
    });

    it('clears the deadline error when the session type changes', async () => {
      const user = userEvent.setup();
      renderPage();
      await screen.findByText(/click "new session"/i);

      const form = await openForm(user);
      await user.type(form.querySelector('input[type="date"]')!, '2026-12-11');
      await user.click(deadlineToggle());
      await user.type(form.querySelector('input[type="datetime-local"]')!, '2020-01-01T18:00');
      await user.click(screen.getByRole('button', { name: /create session/i }));

      expect(await screen.findByText(/cannot be in the past/i)).toBeInTheDocument();

      await user.click(screen.getByRole('radio', { name: /recurring weekly/i }));

      expect(screen.queryByText(/cannot be in the past/i)).not.toBeInTheDocument();
    });
  });
});


describe('Session cancellation', () => {
  it.each(['Court flooded', ''])('confirms cancellation with optional reason %j', async (reason) => {
    const user = userEvent.setup();
    vi.mocked(api.listSessions).mockResolvedValue([makeSession({ is_recurring: true })]);
    vi.mocked(api.cancelSession).mockResolvedValue(makeSession({ status: 'cancelled', cancellation_reason: reason }));
    renderPage();
    await user.click(await screen.findByRole('button', { name: 'Cancel Session' }));
    const dialog = screen.getByRole('dialog', { name: 'Cancel Session' });
    const input = within(dialog).getByLabelText('Reason for cancellation (optional)');
    expect(input).toHaveFocus();
    expect(within(dialog).getByText(/all approved members/i)).toHaveTextContent('Only this date will be cancelled.');
    expect(api.cancelSession).not.toHaveBeenCalled();
    if (reason) await user.type(input, reason);
    await user.click(within(dialog).getByRole('button', { name: 'Cancel Session' }));
    expect(api.cancelSession).toHaveBeenCalledWith('session-1', reason);
    expect(await screen.findByText('Cancelled')).toBeInTheDocument();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.queryByTitle('Delete')).not.toBeInTheDocument();
  });

  it('keeps cancelled sessions and reasons visible after loading', async () => {
    vi.mocked(api.listCancelledSessions).mockResolvedValue([makeSession({ status: 'cancelled', cancellation_reason: 'Court flooded' })]);
    renderPage();
    expect(await screen.findByText('Cancelled')).toBeInTheDocument();
    expect(screen.getByText('Reason: Court flooded')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Cancel Session' })).not.toBeInTheDocument();
  });

  it('shows failures and preserves the reason for retry', async () => {
    const user = userEvent.setup();
    vi.mocked(api.listSessions).mockResolvedValue([makeSession()]);
    vi.mocked(api.cancelSession).mockRejectedValueOnce({ response: { data: { error: 'Session changed. Try again.' } } });
    renderPage();
    await user.click(await screen.findByRole('button', { name: 'Cancel Session' }));
    const dialog = screen.getByRole('dialog');
    await user.type(within(dialog).getByLabelText('Reason for cancellation (optional)'), 'Rain');
    await user.click(within(dialog).getByRole('button', { name: 'Cancel Session' }));
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('Session changed. Try again.');
    expect(within(dialog).getByLabelText('Reason for cancellation (optional)')).toHaveValue('Rain');
    expect(within(dialog).getByRole('button', { name: 'Cancel Session' })).toBeEnabled();
  });

  it('keeps the session when dismissed and restores focus', async () => {
    const user = userEvent.setup();
    vi.mocked(api.listSessions).mockResolvedValue([makeSession()]);
    renderPage();
    const trigger = await screen.findByRole('button', { name: 'Cancel Session' });
    await user.click(trigger);
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
    expect(api.cancelSession).not.toHaveBeenCalled();
  });

  it('prevents duplicate submission and dismissal while cancellation is pending', async () => {
    const user = userEvent.setup();
    vi.mocked(api.listSessions).mockResolvedValue([makeSession()]);
    vi.mocked(api.cancelSession).mockReturnValue(new Promise(() => {}));
    renderPage();
    await user.click(await screen.findByRole('button', { name: 'Cancel Session' }));
    const dialog = screen.getByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Cancel Session' }));
    for (const button of within(dialog).getAllByRole('button')) expect(button).toBeDisabled();
    expect(within(dialog).getByRole('textbox')).toBeDisabled();
    expect(api.cancelSession).toHaveBeenCalledTimes(1);
  });
});
