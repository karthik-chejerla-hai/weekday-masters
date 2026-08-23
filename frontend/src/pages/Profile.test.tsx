import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Profile from './Profile';
import { useAuth } from '../context/useAuth';
import { api } from '../services/api';

vi.mock('../context/useAuth', () => ({ useAuth: vi.fn() }));
vi.mock('../services/api', () => ({
  api: {
    updateMe: vi.fn(),
    getNotificationPreferences: vi.fn(),
    updateNotificationPreferences: vi.fn(),
  },
}));
// The notification panel has its own tests; it is not what this page is about.
vi.mock('../components/notifications/NotificationSettings', () => ({
  default: () => <div>Notification settings</div>,
}));

const refreshUser = vi.fn();

function mockUser(overrides: Record<string, unknown> = {}) {
  vi.mocked(useAuth).mockReturnValue({
    user: {
      name: 'Jane Player',
      email: 'jane@example.com',
      phone_number: '+61400000000',
      profile_picture: '',
      role: 'player',
      membership_status: 'approved',
      ...overrides,
    },
    refreshUser,
  } as unknown as ReturnType<typeof useAuth>);
}

beforeEach(() => {
  vi.clearAllMocks();
  mockUser();
  vi.mocked(api.updateMe).mockResolvedValue({} as never);
});

describe('Profile page', () => {
  it('shows the member details', () => {
    render(<Profile />);

    expect(screen.getByText('Jane Player')).toBeInTheDocument();
    expect(screen.getByDisplayValue('+61400000000')).toBeInTheDocument();

    // Email comes from Google, so the field shows it but must not be editable.
    const email = screen.getByDisplayValue('jane@example.com');
    expect(email).toBeDisabled();
  });

  it('saves an edited phone number and refreshes the session', async () => {
    render(<Profile />);

    const input = screen.getByDisplayValue('+61400000000');
    await userEvent.clear(input);
    await userEvent.type(input, '+61411111111');
    await userEvent.click(screen.getByRole('button', { name: /save/i }));

    expect(api.updateMe).toHaveBeenCalledWith('+61411111111');
    // Without the refresh the header keeps showing stale details.
    expect(refreshUser).toHaveBeenCalledOnce();
    expect(await screen.findByText(/updated successfully/i)).toBeInTheDocument();
  });

  it('reports a failed save instead of claiming success', async () => {
    vi.mocked(api.updateMe).mockRejectedValue(new Error('server error'));

    render(<Profile />);
    await userEvent.click(screen.getByRole('button', { name: /save/i }));

    expect(await screen.findByText(/failed to update profile/i)).toBeInTheDocument();
    expect(refreshUser).not.toHaveBeenCalled();
  });

  it('renders nothing until the user is known', () => {
    vi.mocked(useAuth).mockReturnValue({ user: null, refreshUser } as unknown as ReturnType<typeof useAuth>);

    const { container } = render(<Profile />);
    expect(container).toBeEmptyDOMElement();
  });
});
