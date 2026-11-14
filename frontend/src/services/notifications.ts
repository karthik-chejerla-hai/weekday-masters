import { api, NotificationPreferences, Notification, PushTestResult } from './api';
import {
  onForegroundMessage,
  getNotificationPermission,
  isFirebaseConfigured
} from './firebase';

import { restorePushDevice, enablePushDevice, disablePushDevice, isPushDeviceDisabled } from './push-device';
export type { NotificationPreferences, Notification };

export const notificationService = {
  // Check if push notifications are supported
  isPushSupported(): boolean {
    return (
      'Notification' in window &&
      'serviceWorker' in navigator &&
      isFirebaseConfigured()
    );
  },

  // Get current permission status
  getPermissionStatus(): NotificationPermission | 'unsupported' {
    return getNotificationPermission();
  },

  restorePushNotifications(userID: string): Promise<boolean> { return restorePushDevice(userID); },
  isDeviceDisabled(userID: string): boolean { return isPushDeviceDisabled(userID); },
  enablePushNotifications(userID: string): Promise<boolean> { return enablePushDevice(userID); },
  disablePushNotifications(userID: string): Promise<void> { return disablePushDevice(userID); },

  async sendTestPush(): Promise<PushTestResult> {
    return api.sendTestPush();
  },

  // Set up handler for foreground notifications
  setupForegroundHandler(
    onNotification: (title: string, body: string, data?: Record<string, string>) => void
  ): () => void {
    return onForegroundMessage((payload) => {
      onNotification(
        payload.title || 'Notification',
        payload.body || '',
        payload.data
      );
    });
  },

  // Get user's notification preferences
  async getPreferences(): Promise<NotificationPreferences> {
    return api.getNotificationPreferences();
  },

  // Update user's notification preferences
  async updatePreferences(
    updates: Partial<Omit<NotificationPreferences, 'id' | 'user_id' | 'created_at' | 'updated_at'>>
  ): Promise<NotificationPreferences> {
    const saved = await api.updateNotificationPreferences(updates);
    window.dispatchEvent(new Event('notification-settings-changed'));
    return saved;
  },

  // Get notification history
  async getHistory(limit = 20, offset = 0): Promise<Notification[]> {
    return api.getNotificationHistory(limit, offset);
  },

  // Mark notification as read
  async markAsRead(notificationId: string): Promise<void> {
    return api.markNotificationRead(notificationId);
  }
};
