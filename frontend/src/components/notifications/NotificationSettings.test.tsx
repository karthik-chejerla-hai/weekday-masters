import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import NotificationSettings from './NotificationSettings';
import { notificationService } from '../../services/notifications';

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
    sendTestPush: vi.fn().mockResolvedValue({ accepted_devices: 1, attempted_devices: 1 }),
  },
}));

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(notificationService.isPushSupported).mockReturnValue(true);
  vi.mocked(notificationService.getPermissionStatus).mockReturnValue('granted');
  vi.mocked(notificationService.getPreferences).mockResolvedValue({
    id: 'pref-1', user_id: 'user-1', push_enabled: true,
    push_session_reminders: true, push_rsvp_deadlines: true,
    push_waitlist_updates: true, push_admin_announcements: true,
  } as never);
  vi.mocked(notificationService.updatePreferences).mockResolvedValue({
    id: 'pref-1', user_id: 'user-1', push_enabled: false,
    push_session_reminders: true, push_rsvp_deadlines: true,
    push_waitlist_updates: true, push_admin_announcements: true,
  } as never);
  vi.mocked(notificationService.enablePushNotifications).mockResolvedValue(true);
  vi.mocked(notificationService.sendTestPush).mockResolvedValue({ accepted_devices: 1, attempted_devices: 1 });
});

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
  await screen.findByText('Enable on this device');
  expect(notificationService.enablePushNotifications).toHaveBeenCalled();
  expect(screen.queryByText('Email Notifications')).not.toBeInTheDocument();
});

it('reports a settings save failure without claiming push is enabled', async () => {
  const { notificationService } = await import('../../services/notifications');
  vi.mocked(notificationService.enablePushNotifications).mockResolvedValueOnce(false).mockResolvedValueOnce(true);
  vi.mocked(notificationService.updatePreferences).mockRejectedValueOnce(new Error('offline'));
  render(<NotificationSettings />);
  const button = await screen.findByText('Enable on this device');
  fireEvent.click(button);
  await screen.findByText('Failed to enable push notifications');
  expect(screen.queryByText('Push notifications enabled on this device.')).not.toBeInTheDocument();
});

it('reports the device as ready only after token registration and the account preference save', async () => {
  const { notificationService } = await import('../../services/notifications');
  vi.mocked(notificationService.getPermissionStatus).mockReturnValueOnce('default').mockReturnValue('granted');
  vi.mocked(notificationService.getPreferences).mockResolvedValueOnce({ push_enabled: false } as never);
  vi.mocked(notificationService.updatePreferences).mockResolvedValueOnce({ push_enabled: true } as never);
  render(<NotificationSettings />);
  fireEvent.click(await screen.findByText('Enable on this device'));
  await screen.findByText('This device is registered for push notifications');
  expect(screen.getByText('Push notifications enabled on this device.')).toBeInTheDocument();
  expect(notificationService.updatePreferences).toHaveBeenCalledWith({ push_enabled: true });
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

it('lets an admin send a self-targeted push test after device setup', async () => {
  const { notificationService } = await import('../../services/notifications');
  render(<NotificationSettings isAdmin />);
  fireEvent.click(await screen.findByRole('button', { name: 'Send test notification' }));
  await screen.findByText('Test notification accepted by 1 of 1 registered device.');
  expect(notificationService.sendTestPush).toHaveBeenCalledOnce();
});

it('does not expose the push test to members', async () => {
  render(<NotificationSettings />);
  await screen.findByText('This device is registered for push notifications');
  expect(screen.queryByRole('button', { name: 'Send test notification' })).not.toBeInTheDocument();
});
