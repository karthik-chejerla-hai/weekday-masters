import { api, NotificationPreferences, Notification } from './api';
import {
  requestNotificationPermission,
  onForegroundMessage,
  getNotificationPermission,
  isFirebaseConfigured
} from './firebase';

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

  // Enable push notifications
  async enablePushNotifications(): Promise<boolean> {
    try {
      const token = await requestNotificationPermission();
      if (!token) {
        return false;
      }

      // Register token with backend
      await api.registerPushToken(token);
      return true;
    } catch (error) {
      console.error('Failed to enable push notifications:', error);
      return false;
    }
  },

  // Disable push notifications
  async disablePushNotifications(): Promise<void> {
    try {
      await api.unregisterPushToken();
    } catch (error) {
      console.error('Failed to disable push notifications:', error);
    }
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
    return api.updateNotificationPreferences(updates);
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
