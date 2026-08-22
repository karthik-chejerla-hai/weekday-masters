import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import PendingApproval from './PendingApproval';
import { useAuth } from '../context/useAuth';

vi.mock('../context/useAuth', () => ({ useAuth: vi.fn() }));

const logout = vi.fn();

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(useAuth).mockReturnValue({
    user: { name: 'Jane Player', email: 'jane@example.com', profile_picture: '' },
    logout,
  } as unknown as ReturnType<typeof useAuth>);
});

describe('PendingApproval page', () => {
  it('tells the member their membership is awaiting approval', () => {
    render(<PendingApproval />);

    expect(screen.getByText('Membership Pending')).toBeInTheDocument();
    expect(screen.getByText('Jane Player')).toBeInTheDocument();
    expect(screen.getByText('jane@example.com')).toBeInTheDocument();
  });

  it('lets them sign out while they wait', async () => {
    render(<PendingApproval />);

    await userEvent.click(screen.getByRole('button', { name: /sign out/i }));
    expect(logout).toHaveBeenCalledOnce();
  });

  it('renders without a user rather than crashing', () => {
    vi.mocked(useAuth).mockReturnValue({ user: null, logout } as unknown as ReturnType<typeof useAuth>);

    render(<PendingApproval />);
    expect(screen.getByText('Membership Pending')).toBeInTheDocument();
  });
});
