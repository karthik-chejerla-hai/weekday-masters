import { act, render, screen, waitFor } from '@testing-library/react';
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
