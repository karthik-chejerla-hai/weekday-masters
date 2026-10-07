import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import NotificationSettings from './NotificationSettings';

vi.mock('../../services/notifications', () => ({
  notificationService: {
    isPushSupported: vi.fn().mockReturnValue(true),
    getPermissionStatus: vi.fn().mockReturnValue('granted'),
    getPreferences: vi.fn().mockResolvedValue({
      id: 'pref-1',
      user_id: 'user-1',
      push_enabled: true,
      push_session_reminders: true,
      push_rsvp_deadlines: true,
      push_waitlist_updates: true,
      push_admin_announcements: true,
      email_enabled: true,
      email_session_reminders: true,
      email_rsvp_deadlines: true,
      email_waitlist_updates: true,
      email_admin_announcements: true,
    }),
    updatePreferences: vi.fn().mockResolvedValue({
      id: 'pref-1',
      user_id: 'user-1',
      push_enabled: false,
      push_session_reminders: true,
      push_rsvp_deadlines: true,
      push_waitlist_updates: true,
      push_admin_announcements: true,
      email_enabled: true,
      email_session_reminders: true,
      email_rsvp_deadlines: true,
      email_waitlist_updates: true,
      email_admin_announcements: true,
    }),
    enablePushNotifications: vi.fn().mockResolvedValue(true),
  },
}));

beforeEach(() => { vi.clearAllMocks(); });

describe('NotificationSettings Component', () => {

  it('loads and renders notification settings', async () => {
    render(<NotificationSettings />);

    await waitFor(() => {
      expect(screen.getAllByText('Push Notifications').length).toBeGreaterThan(0);
      expect(screen.getByText('Session Reminders')).toBeInTheDocument();
      expect(screen.getByText('RSVP Deadlines')).toBeInTheDocument();
      expect(screen.getByText('Waitlist Updates')).toBeInTheDocument();
      expect(screen.getByText('Club Announcements')).toBeInTheDocument();
    });
  });
});

it('repairs registration even when browser permission is already granted', async () => {
  const { notificationService } = await import('../../services/notifications');
  vi.mocked(notificationService.enablePushNotifications).mockResolvedValueOnce(false);
  render(<NotificationSettings />);
  await screen.findByText('Set up this device');
  expect(notificationService.enablePushNotifications).toHaveBeenCalled();
  expect(screen.queryByText('Email Notifications')).not.toBeInTheDocument();
});

it('reports a settings save failure without claiming push is enabled', async () => {
  const { notificationService } = await import('../../services/notifications');
  vi.mocked(notificationService.enablePushNotifications).mockResolvedValueOnce(false).mockResolvedValueOnce(true);
  vi.mocked(notificationService.updatePreferences).mockRejectedValueOnce(new Error('offline'));
  render(<NotificationSettings />);
  const button = await screen.findByText('Set up this device');
  fireEvent.click(button);
  await screen.findByText('Failed to enable push notifications');
  expect(screen.queryByText('This device is registered for push alerts.')).not.toBeInTheDocument();
});

 it.each(['registration fails', 'permission denied', 'unsupported browser'])(
  'can disable account alerts when %s', async (mode) => {
    const { notificationService } = await import('../../services/notifications');
    if (mode === 'registration fails') vi.mocked(notificationService.enablePushNotifications).mockResolvedValueOnce(false);
    if (mode === 'permission denied') vi.mocked(notificationService.getPermissionStatus).mockReturnValueOnce('denied').mockReturnValueOnce('denied');
    if (mode === 'unsupported browser') vi.mocked(notificationService.isPushSupported).mockReturnValueOnce(false);
    render(<NotificationSettings />);
    const toggle = await screen.findByRole('switch', { name: 'Account push alerts' });
    expect(toggle).toHaveAttribute('aria-checked', 'true');
    const registrationCalls = vi.mocked(notificationService.enablePushNotifications).mock.calls.length;
    fireEvent.click(toggle);
    await waitFor(() => expect(toggle).toHaveAttribute('aria-checked', 'false'));
    expect(notificationService.updatePreferences).toHaveBeenCalledWith({ push_enabled: false });
    expect(notificationService.enablePushNotifications).toHaveBeenCalledTimes(registrationCalls);
  }
);


it('lets the member consent to private WhatsApp balance alerts', async () => {
  const { notificationService } = await import('../../services/notifications');
  render(<NotificationSettings />);
  const toggle = await screen.findByRole('switch', { name: 'WhatsApp balance alerts' });
  expect(toggle).toHaveAttribute('aria-checked', 'false');
  fireEvent.click(toggle);
  await waitFor(() => expect(notificationService.updatePreferences).toHaveBeenCalledWith({ whatsapp_balance_alerts: true }));
  expect(screen.getByText(/no device confirms receipt within 15 minutes/)).toBeInTheDocument();
});


it('does not show selected alert types as ready when token registration fails', async () => {
  const { notificationService } = await import('../../services/notifications');
  vi.mocked(notificationService.enablePushNotifications).mockResolvedValueOnce(false);
  render(<NotificationSettings />);
  await screen.findByRole('button', { name: 'Set up this device' });
  expect(screen.getByText('Push alerts are not ready on this device')).toBeInTheDocument();
  expect(screen.queryByRole('switch', { name: 'Session Reminders' })).not.toBeInTheDocument();
  expect(screen.getByText(/A saved On preference does not confirm device setup/)).toBeInTheDocument();
});

it('turning the account preference on also registers this device', async () => {
  const { notificationService } = await import('../../services/notifications');
  vi.mocked(notificationService.getPreferences).mockResolvedValueOnce({ push_enabled: false } as never);
  vi.mocked(notificationService.updatePreferences).mockResolvedValueOnce({ push_enabled: true } as never);
  render(<NotificationSettings />);
  const toggle = await screen.findByRole('switch', { name: 'Account push alerts' });
  fireEvent.click(toggle);
  await screen.findByText('This device is registered');
  expect(notificationService.enablePushNotifications).toHaveBeenCalledOnce();
  expect(notificationService.updatePreferences).toHaveBeenCalledWith({ push_enabled: true });
});
