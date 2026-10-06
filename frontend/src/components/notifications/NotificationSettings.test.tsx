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

describe('NotificationSettings Component', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

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
  await screen.findByText('Enable Push Notifications');
  expect(notificationService.enablePushNotifications).toHaveBeenCalled();
  expect(screen.queryByText('Email Notifications')).not.toBeInTheDocument();
});

it('reports a settings save failure without claiming push is enabled', async () => {
  const { notificationService } = await import('../../services/notifications');
  vi.mocked(notificationService.enablePushNotifications).mockResolvedValueOnce(false).mockResolvedValueOnce(true);
  vi.mocked(notificationService.updatePreferences).mockRejectedValueOnce(new Error('offline'));
  render(<NotificationSettings />);
  const button = await screen.findByText('Enable Push Notifications');
  fireEvent.click(button);
  await screen.findByText('Failed to enable push notifications');
  expect(screen.queryByText('Push notifications enabled!')).not.toBeInTheDocument();
});
