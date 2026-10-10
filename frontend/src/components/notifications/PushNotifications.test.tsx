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
  updatePreferences: vi.fn(async () => ({ push_enabled: true })),
  setupForegroundHandler: vi.fn(() => vi.fn()),
} }));

const storedValues = new Map<string, string>();
Object.defineProperty(window, 'localStorage', {
  configurable: true,
  value: {
    getItem: (key: string) => storedValues.get(key) ?? null,
    setItem: (key: string, value: string) => storedValues.set(key, value),
    removeItem: (key: string) => storedValues.delete(key),
    clear: () => storedValues.clear(),
  },
});

beforeEach(() => {
  vi.clearAllMocks();
  window.localStorage.clear();
  vi.mocked(notificationService.isPushSupported).mockReturnValue(true);
  vi.mocked(notificationService.getPermissionStatus).mockReturnValue('granted');
  vi.mocked(notificationService.getPreferences).mockResolvedValue({ push_enabled: true } as never);
  vi.mocked(notificationService.enablePushNotifications).mockResolvedValue(true);
  vi.mocked(notificationService.updatePreferences).mockResolvedValue({ push_enabled: true } as never);
  vi.mocked(notificationService.setupForegroundHandler).mockReturnValue(vi.fn());
});

function renderPush(path = '/dashboard') {
  return render(<MemoryRouter initialEntries={[path]}><PushNotifications /></MemoryRouter>);
}

it('restores registration and shows an actionable foreground message', async () => {
  renderPush();
  await waitFor(() => expect(notificationService.setupForegroundHandler).toHaveBeenCalled());
  expect(notificationService.enablePushNotifications).toHaveBeenCalledOnce();
  const callback = vi.mocked(notificationService.setupForegroundHandler).mock.calls[0][0];
  act(() => callback('Session reminder', 'Time to play', { session_id: 's1' }));
  expect(screen.getByRole('link')).toHaveAttribute('href', '/sessions/s1');
});

it('offers an explicit enable action without requesting permission on page load', async () => {
  vi.mocked(notificationService.getPermissionStatus).mockReturnValueOnce('default');
  vi.mocked(notificationService.getPreferences).mockResolvedValueOnce({ push_enabled: false } as never);
  renderPush();
  expect(await screen.findByRole('button', { name: 'Enable on this device' })).toBeInTheDocument();
  expect(notificationService.enablePushNotifications).not.toHaveBeenCalled();
  expect(notificationService.setupForegroundHandler).not.toHaveBeenCalled();
});

it('registers the device and enables account push after the member opts in', async () => {
  vi.mocked(notificationService.getPermissionStatus).mockReturnValueOnce('default');
  vi.mocked(notificationService.getPreferences).mockResolvedValueOnce({ push_enabled: false } as never);
  renderPush();
  fireEvent.click(await screen.findByRole('button', { name: 'Enable on this device' }));
  await screen.findByText('Push notifications enabled');
  expect(notificationService.enablePushNotifications).toHaveBeenCalledOnce();
  expect(notificationService.updatePreferences).toHaveBeenCalledWith({ push_enabled: true });
});

it('remembers Not now for seven days without changing notification preferences', async () => {
  vi.mocked(notificationService.getPermissionStatus).mockReturnValue('default');
  vi.mocked(notificationService.getPreferences).mockResolvedValue({ push_enabled: false } as never);
  const first = renderPush();
  fireEvent.click(await screen.findByRole('button', { name: 'Not now' }));
  expect(screen.queryByRole('region', { name: 'Push notification setup' })).not.toBeInTheDocument();
  first.unmount();

  renderPush();
  await waitFor(() => expect(notificationService.getPreferences).toHaveBeenCalledTimes(2));
  expect(screen.queryByRole('region', { name: 'Push notification setup' })).not.toBeInTheDocument();
  expect(notificationService.updatePreferences).not.toHaveBeenCalled();
});

it('offers the prompt again after the seven-day dismissal expires', async () => {
  const clock = vi.spyOn(Date, 'now').mockReturnValue(Date.parse('2026-10-10T00:00:00Z'));
  vi.mocked(notificationService.getPermissionStatus).mockReturnValue('default');
  vi.mocked(notificationService.getPreferences).mockResolvedValue({ push_enabled: false } as never);
  try {
    const first = renderPush();
    fireEvent.click(await screen.findByRole('button', { name: 'Not now' }));
    first.unmount();

    clock.mockReturnValue(Date.parse('2026-10-18T00:00:00Z'));
    renderPush();
    expect(await screen.findByRole('button', { name: 'Enable on this device' })).toBeInTheDocument();
  } finally {
    clock.mockRestore();
  }
});

it('does not show the dashboard prompt on other pages', async () => {
  vi.mocked(notificationService.getPermissionStatus).mockReturnValueOnce('default');
  vi.mocked(notificationService.getPreferences).mockResolvedValueOnce({ push_enabled: false } as never);
  renderPush('/sessions');
  await waitFor(() => expect(notificationService.getPreferences).toHaveBeenCalled());
  expect(screen.queryByRole('region', { name: 'Push notification setup' })).not.toBeInTheDocument();
});

it('routes blocked devices to notification settings', async () => {
  vi.mocked(notificationService.getPermissionStatus).mockReturnValueOnce('denied');
  renderPush();
  expect(await screen.findByRole('link', { name: 'Notification settings' })).toHaveAttribute('href', '/profile#push-notifications');
  expect(notificationService.enablePushNotifications).not.toHaveBeenCalled();
});

it('removes the foreground listener when preferences are disabled', async () => {
  const unsubscribe = vi.fn();
  vi.mocked(notificationService.setupForegroundHandler).mockReturnValueOnce(unsubscribe);
  renderPush();
  await waitFor(() => expect(notificationService.setupForegroundHandler).toHaveBeenCalled());
  vi.mocked(notificationService.getPreferences).mockResolvedValueOnce({ push_enabled: false } as never);
  act(() => window.dispatchEvent(new Event('notification-settings-changed')));
  await waitFor(() => expect(unsubscribe).toHaveBeenCalled());
  expect(notificationService.enablePushNotifications).toHaveBeenCalledOnce();
});

it.each(['balance_low', 'balance_negative'])('opens Money for a %s foreground alert', async (type) => {
  renderPush();
  await waitFor(() => expect(notificationService.setupForegroundHandler).toHaveBeenCalled());
  const callback = vi.mocked(notificationService.setupForegroundHandler).mock.calls[0][0];
  act(() => callback('Balance alert', 'Check your balance', { type, balance_cents: '100' }));
  expect(screen.getByRole('link')).toHaveAttribute('href', '/money');
});
