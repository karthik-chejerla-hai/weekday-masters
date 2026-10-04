import { beforeEach, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { useAuth } from '../context/useAuth';
import Welcome from './Welcome';

vi.mock('../context/useAuth', () => ({ useAuth: vi.fn() }));
const login = vi.fn();
const retry = vi.fn();
beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(useAuth).mockReturnValue({ isAuthenticated: false, isApproved: false, user: null, login, loginForInvitation: login, retryAuth: retry } as unknown as ReturnType<typeof useAuth>);
});
function show() { render(<MemoryRouter><Welcome /></MemoryRouter>); }
it('shows the correct account instructions and starts Google sign-in', async () => {
  show();
  expect(screen.getByText(/Google email address shown in your invitation/)).toBeInTheDocument();
  expect(screen.getByText(/do not need to RSVP again/)).toBeInTheDocument();
  await userEvent.click(screen.getByRole('button', { name: 'Sign in with Google' }));
  expect(login).toHaveBeenCalledOnce();
});
it('confirms approved access and links to the member dashboard', () => {
  vi.mocked(useAuth).mockReturnValue({ isAuthenticated: true, isApproved: true, user: { email: 'hari@example.com' }, login } as unknown as ReturnType<typeof useAuth>);
  show();
  expect(screen.getByText('hari@example.com')).toBeInTheDocument();
  expect(screen.getByRole('link', { name: 'Open my club' })).toHaveAttribute('href', '/dashboard');
});
it('helps a different Google account without claiming the invited account', async () => {
  vi.mocked(useAuth).mockReturnValue({ isAuthenticated: true, isApproved: false, user: { email: 'different@example.com' }, login } as unknown as ReturnType<typeof useAuth>);
  show();
  expect(screen.getByRole('alert')).toHaveTextContent('different@example.com does not have approved club access');
  expect(screen.queryByRole('link', { name: 'Open my club' })).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole('button', { name: 'Choose another Google account' }));
  expect(login).toHaveBeenCalledOnce();
});
it('shows a sign-in failure and permits a retry', async () => {
  vi.mocked(useAuth).mockReturnValue({ isAuthenticated: false, isApproved: false, login, authError: 'Could not verify your profile.', retryAuth: retry } as unknown as ReturnType<typeof useAuth>);
  show();
  expect(screen.getByRole('alert')).toHaveTextContent('Could not verify your profile.');
  await userEvent.click(screen.getByRole('button', { name: 'Try again' }));
  expect(retry).toHaveBeenCalledOnce();
});
