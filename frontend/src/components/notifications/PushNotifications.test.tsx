import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, expect, it, vi } from 'vitest';
import PushNotifications from './PushNotifications';
import { notificationService } from '../../services/notifications';

vi.mock('../../context/useAuth', () => ({ useAuth: () => ({ user: { id: 'member' }, isApproved: true }) }));
vi.mock('../../services/notifications', () => ({ notificationService: {
  isPushSupported: vi.fn(() => true),
  getPermissionStatus: vi.fn(() => 'granted'),
  getPreferences: vi.fn(async () => ({ push_enabled: true })),
  enablePushNotifications: vi.fn(async () => true),
  setupForegroundHandler: vi.fn(() => vi.fn()),
} }));
beforeEach(() => vi.clearAllMocks());

it('restores registration and shows an actionable foreground message', async () => {
  render(<MemoryRouter><PushNotifications /></MemoryRouter>);
  await waitFor(() => expect(notificationService.setupForegroundHandler).toHaveBeenCalled());
  expect(notificationService.enablePushNotifications).toHaveBeenCalledOnce();
  const callback = vi.mocked(notificationService.setupForegroundHandler).mock.calls[0][0];
  act(() => callback('Session reminder', 'Time to play', { session_id: 's1' }));
  expect(screen.getByRole('link')).toHaveAttribute('href', '/sessions/s1');
});

it('does not request permission when permission has not been granted', async () => {
  vi.mocked(notificationService.getPermissionStatus).mockReturnValueOnce('default');
  render(<MemoryRouter><PushNotifications /></MemoryRouter>);
  await waitFor(() => expect(notificationService.getPermissionStatus).toHaveBeenCalled());
  expect(notificationService.enablePushNotifications).not.toHaveBeenCalled();
  expect(notificationService.setupForegroundHandler).not.toHaveBeenCalled();
});

it('removes the foreground listener when preferences are disabled', async () => {
  const unsubscribe = vi.fn();
  vi.mocked(notificationService.setupForegroundHandler).mockReturnValueOnce(unsubscribe);
  render(<MemoryRouter><PushNotifications /></MemoryRouter>);
  await waitFor(() => expect(notificationService.setupForegroundHandler).toHaveBeenCalled());
  vi.mocked(notificationService.getPreferences).mockResolvedValueOnce({ push_enabled: false } as never);
  act(() => window.dispatchEvent(new Event('notification-settings-changed')));
  await waitFor(() => expect(unsubscribe).toHaveBeenCalled());
  expect(notificationService.enablePushNotifications).toHaveBeenCalledOnce();
});

it.each(['balance_low', 'balance_negative'])('opens Money for a %s foreground alert', async (type) => {
  render(<MemoryRouter><PushNotifications /></MemoryRouter>);
  await waitFor(() => expect(notificationService.setupForegroundHandler).toHaveBeenCalled());
  const callback = vi.mocked(notificationService.setupForegroundHandler).mock.calls[0][0];
  act(() => callback('Balance alert', 'Check your balance', { type, balance_cents: '100' }));
  expect(screen.getByRole('link')).toHaveAttribute('href', '/money');
});


it('prompts for setup without requesting browser permission on page load', async () => {
  vi.mocked(notificationService.getPermissionStatus).mockReturnValueOnce('default');
  render(<MemoryRouter><PushNotifications /></MemoryRouter>);
  expect(await screen.findByRole('link', { name: 'Set up push alerts' })).toHaveAttribute('href', '/profile#push-notifications');
  expect(notificationService.enablePushNotifications).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: 'Not now' }));
  expect(screen.queryByRole('region', { name: 'Push notification setup' })).not.toBeInTheDocument();
});

it('does not treat failed registration as working push', async () => {
  vi.mocked(notificationService.enablePushNotifications).mockResolvedValueOnce(false);
  render(<MemoryRouter><PushNotifications /></MemoryRouter>);
  await screen.findByText('We could not register this device. Open settings to try again.');
  expect(notificationService.setupForegroundHandler).not.toHaveBeenCalled();
});

it('does not prompt members who turned off account push alerts', async () => {
  vi.mocked(notificationService.getPreferences).mockResolvedValueOnce({ push_enabled: false } as never);
  render(<MemoryRouter><PushNotifications /></MemoryRouter>);
  await waitFor(() => expect(notificationService.getPreferences).toHaveBeenCalled());
  expect(screen.queryByRole('region', { name: 'Push notification setup' })).not.toBeInTheDocument();
  expect(notificationService.enablePushNotifications).not.toHaveBeenCalled();
});

it('clears the setup prompt after successful registration on returning to the app', async () => {
  vi.mocked(notificationService.enablePushNotifications).mockResolvedValueOnce(false);
  render(<MemoryRouter><PushNotifications /></MemoryRouter>);
  await screen.findByRole('region', { name: 'Push notification setup' });
  act(() => window.dispatchEvent(new Event('focus')));
  await waitFor(() => expect(notificationService.setupForegroundHandler).toHaveBeenCalled());
  expect(screen.queryByRole('region', { name: 'Push notification setup' })).not.toBeInTheDocument();
});
