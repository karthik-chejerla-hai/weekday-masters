import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import AdminMembers from './AdminMembers';
import { api } from '../services/api';
import type { User } from '../types';

vi.mock('../services/api', () => ({
  api: {
    adminListMembers: vi.fn(),
    inviteMember: vi.fn(),
    updateMember: vi.fn(),
    removeMember: vi.fn(),
    reinstateMember: vi.fn(),
  },
}));

function makeMember(overrides: Partial<User> = {}): User {
  return {
    id: 'user-1',
    auth0_id: 'google-oauth2|1',
    email: 'signed.in@example.com',
    name: 'Signed In',
    nickname: '',
    profile_picture: '',
    phone_number: '',
    role: 'player',
    is_player: true,
    membership_status: 'approved',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

/** A member an admin added who has never signed in. */
function makeInvited(overrides: Partial<User> = {}): User {
  return makeMember({
    id: 'user-2',
    auth0_id: 'invite:6f1c9e2a-0000-4000-8000-000000000000',
    email: 'invited@example.com',
    name: 'Not Yet Here',
    ...overrides,
  });
}

/** An error shaped the way the member endpoints answer failures. */
function apiError(message: string) {
  return { response: { data: { error: message } } };
}

function renderPage() {
  return render(
    <MemoryRouter>
      <AdminMembers />
    </MemoryRouter>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.adminListMembers).mockResolvedValue([]);
});

describe('the roll', () => {
  it('separates signed-in members from invites who have not arrived yet', async () => {
    vi.mocked(api.adminListMembers).mockResolvedValue([makeMember(), makeInvited()]);
    const user = userEvent.setup();
    renderPage();

    expect(await screen.findByText('Signed')).toBeInTheDocument();
    expect(screen.queryByText('Not')).not.toBeInTheDocument();

    await user.click(screen.getByRole('tab', { name: /invited \(1\)/i }));
    expect(screen.getByText('Not')).toBeInTheDocument();
    expect(screen.getByText(/not signed in/i)).toBeInTheDocument();
  });

  it('leaves join requests to the approval queue rather than listing them here', async () => {
    vi.mocked(api.adminListMembers).mockResolvedValue([
      makeMember(),
      makeMember({ id: 'user-3', name: 'Still Queueing', membership_status: 'pending' }),
    ]);
    renderPage();

    expect(await screen.findByText('Signed')).toBeInTheDocument();
    expect(screen.queryByText('Still')).not.toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /members \(1\)/i })).toBeInTheDocument();
  });

  it('shows the nickname the club uses, with the full name alongside it', async () => {
    vi.mocked(api.adminListMembers).mockResolvedValue([
      makeMember({ name: 'Priya Raman', nickname: 'Pri' }),
    ]);
    renderPage();

    expect(await screen.findByText('Pri')).toBeInTheDocument();
    expect(screen.getByText('Priya Raman')).toBeInTheDocument();
  });

  it('shows the first name by default, still with the full name to identify them by', async () => {
    vi.mocked(api.adminListMembers).mockResolvedValue([makeMember({ name: 'Priya Raman' })]);
    renderPage();

    expect(await screen.findByText('Priya')).toBeInTheDocument();
    expect(screen.getByText('Priya Raman')).toBeInTheDocument();
  });

  it('searches across name, nickname and email', async () => {
    vi.mocked(api.adminListMembers).mockResolvedValue([
      makeMember({ name: 'Priya Raman', nickname: 'Pri' }),
      makeMember({ id: 'user-9', name: 'Wei Zhang', email: 'wei@example.com' }),
    ]);
    const user = userEvent.setup();
    renderPage();

    await screen.findByText('Pri');
    await user.type(screen.getByRole('searchbox'), 'wei@');

    expect(screen.getByText('Wei')).toBeInTheDocument();
    expect(screen.queryByText('Pri')).not.toBeInTheDocument();
  });
});

describe('adding a member', () => {
  it('adds them and says what has to happen next', async () => {
    const invited = makeInvited({ nickname: 'Wei' });
    vi.mocked(api.inviteMember).mockResolvedValue(invited);
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole('button', { name: /add member/i }));
    const form = screen.getByRole('form', { name: /add a member/i });
    await user.type(within(form).getByLabelText(/email/i), 'invited@example.com');
    await user.type(within(form).getByLabelText(/full name/i), 'Not Yet Here');
    await user.type(within(form).getByLabelText(/nickname/i), 'Wei');
    await user.click(within(form).getByRole('button', { name: /^add member$/i }));

    await waitFor(() =>
      expect(api.inviteMember).toHaveBeenCalledWith({
        email: 'invited@example.com',
        name: 'Not Yet Here',
        nickname: 'Wei',
        phone_number: '',
        role: 'player',
      })
    );

    // The invite only means something once they sign in, so the page says so.
    expect(await screen.findByRole('status')).toHaveTextContent(/invited@example.com/);
    // And it drops them where they now live.
    expect(screen.getByRole('tab', { name: /invited \(1\)/i })).toHaveAttribute(
      'aria-selected',
      'true'
    );
  });

  it("shows the server's own words when the email is already taken", async () => {
    vi.mocked(api.inviteMember).mockRejectedValue(
      apiError('taken@example.com is already a removed member — reinstate them instead')
    );
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole('button', { name: /add member/i }));
    const form = screen.getByRole('form', { name: /add a member/i });
    await user.type(within(form).getByLabelText(/email/i), 'taken@example.com');
    await user.type(within(form).getByLabelText(/full name/i), 'Impostor');
    await user.click(within(form).getByRole('button', { name: /^add member$/i }));

    expect(await screen.findByRole('alert')).toHaveTextContent(/reinstate them instead/);
  });
});

describe('editing a member', () => {
  it('sends only the editable fields and keeps the email out of it for a signed-in member', async () => {
    vi.mocked(api.adminListMembers).mockResolvedValue([makeMember()]);
    vi.mocked(api.updateMember).mockResolvedValue(makeMember({ nickname: 'Smash' }));
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole('button', { name: /edit signed/i }));

    const email = screen.getByLabelText(/email/i);
    expect(email).toBeDisabled();
    expect(screen.getByText(/set by their google sign-in/i)).toBeInTheDocument();

    await user.type(screen.getByLabelText(/nickname/i), 'Smash');
    await user.click(screen.getByRole('button', { name: /save/i }));

    await waitFor(() =>
      expect(api.updateMember).toHaveBeenCalledWith('user-1', {
        name: 'Signed In',
        nickname: 'Smash',
        phone_number: '',
        role: 'player',
      })
    );
    expect(await screen.findByRole('status')).toHaveTextContent(/details were updated/i);
  });

  it('lets an unclaimed invite have its email corrected', async () => {
    vi.mocked(api.adminListMembers).mockResolvedValue([makeInvited()]);
    vi.mocked(api.updateMember).mockResolvedValue(makeInvited({ email: 'fixed@example.com' }));
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole('tab', { name: /invited/i }));
    await user.click(await screen.findByRole('button', { name: /edit not/i }));

    const email = screen.getByLabelText(/email/i);
    expect(email).toBeEnabled();
    await user.clear(email);
    await user.type(email, 'fixed@example.com');
    await user.click(screen.getByRole('button', { name: /save/i }));

    await waitFor(() =>
      expect(api.updateMember).toHaveBeenCalledWith(
        'user-2',
        expect.objectContaining({ email: 'fixed@example.com' })
      )
    );
  });

  it('keeps the form open with the reason when the save is refused', async () => {
    vi.mocked(api.adminListMembers).mockResolvedValue([
      makeMember({ role: 'admin', name: 'Onlyadmin Person' }),
    ]);
    vi.mocked(api.updateMember).mockRejectedValue(
      apiError("this is the club's only admin — promote someone else first")
    );
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole('button', { name: /edit onlyadmin/i }));
    const form = screen.getByRole('form', { name: /edit onlyadmin/i });
    await user.selectOptions(within(form).getByLabelText(/role/i), 'player');
    await user.click(within(form).getByRole('button', { name: /save/i }));

    expect(await screen.findByRole('alert')).toHaveTextContent(/only admin/i);
    expect(screen.getByRole('form', { name: /edit onlyadmin/i })).toBeInTheDocument();
  });
});

describe('removing and reinstating', () => {
  it('removes a member and says the history is kept', async () => {
    vi.mocked(api.adminListMembers).mockResolvedValue([makeMember()]);
    vi.mocked(api.removeMember).mockResolvedValue(makeMember({ membership_status: 'removed' }));
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole('button', { name: /remove signed/i }));

    await waitFor(() => expect(api.removeMember).toHaveBeenCalledWith('user-1'));
    expect(await screen.findByRole('status')).toHaveTextContent(/history is kept/i);

    // They move to the Removed tab rather than disappearing.
    expect(screen.getByRole('tab', { name: /members \(0\)/i })).toBeInTheDocument();
    await user.click(screen.getByRole('tab', { name: /removed \(1\)/i }));
    expect(screen.getByText('Signed')).toBeInTheDocument();
  });

  it('surfaces the refusal when the member still has money with the club', async () => {
    vi.mocked(api.adminListMembers).mockResolvedValue([makeMember()]);
    vi.mocked(api.removeMember).mockRejectedValue(
      apiError('Signed In still has a balance of $25.00 — settle up before removing them')
    );
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole('button', { name: /remove signed/i }));

    expect(await screen.findByRole('alert')).toHaveTextContent(/\$25\.00/);
    // Nothing moved, so they are still a member.
    expect(screen.getByRole('tab', { name: /members \(1\)/i })).toBeInTheDocument();
  });

  it('puts a removed member back', async () => {
    vi.mocked(api.adminListMembers).mockResolvedValue([
      makeMember({ membership_status: 'removed' }),
    ]);
    vi.mocked(api.reinstateMember).mockResolvedValue(makeMember());
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole('tab', { name: /removed \(1\)/i }));
    await user.click(screen.getByRole('button', { name: /reinstate signed/i }));

    await waitFor(() => expect(api.reinstateMember).toHaveBeenCalledWith('user-1'));
    expect(await screen.findByRole('status')).toHaveTextContent(/back in the club/i);
    expect(screen.getByRole('tab', { name: /members \(1\)/i })).toBeInTheDocument();
  });
});

it('reports a failure to load rather than showing an empty club', async () => {
  vi.mocked(api.adminListMembers).mockRejectedValue(apiError('nope'));
  renderPage();

  expect(await screen.findByRole('alert')).toHaveTextContent('nope');
});
