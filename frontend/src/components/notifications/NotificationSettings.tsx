import { useState, useEffect, useCallback } from 'react';
import { Bell, Loader2, BellOff, BellRing, Send, Smartphone } from 'lucide-react';
import { notificationService, NotificationPreferences } from '../../services/notifications';

interface ToggleSwitchProps {
  label?: string;
  enabled: boolean;
  onChange: (enabled: boolean) => void;
  disabled?: boolean;
}

function ToggleSwitch({ label, enabled, onChange, disabled }: ToggleSwitchProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-label={label}
      aria-checked={enabled}
      disabled={disabled}
      onClick={() => onChange(!enabled)}
      className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 ${
        enabled ? 'bg-primary-600' : 'bg-slate-200'
      } ${disabled ? 'opacity-50 cursor-not-allowed' : ''}`}
    >
      <span
        className={`pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${
          enabled ? 'translate-x-5' : 'translate-x-0'
        }`}
      />
    </button>
  );
}

interface NotificationRowProps {
  label: string;
  description: string;
  pushEnabled: boolean;
  onPushChange: (enabled: boolean) => void;
  pushDisabled?: boolean;
}

function NotificationRow({
  label,
  description,
  pushEnabled,
  onPushChange,
  pushDisabled,
}: NotificationRowProps) {
  return (
    <div className="flex items-center justify-between py-4 border-b border-slate-100 last:border-0">
      <div className="flex-1 min-w-0 pr-4">
        <p className="text-sm font-medium text-slate-900">{label}</p>
        <p className="text-xs text-slate-500">{description}</p>
      </div>
      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2">
          <Smartphone className="w-4 h-4 text-slate-400" />
          <ToggleSwitch
            label={label}
            enabled={pushEnabled}
            onChange={onPushChange}
            disabled={pushDisabled}
          />
        </div>
      </div>
    </div>
  );
}

interface NotificationSettingsProps {
  isAdmin?: boolean;
}

export default function NotificationSettings({ isAdmin = false }: NotificationSettingsProps) {
  const [preferences, setPreferences] = useState<NotificationPreferences | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [isSendingTest, setIsSendingTest] = useState(false);
  const [deviceRegistered, setDeviceRegistered] = useState(false);
  const [pushSupported] = useState(() => notificationService.isPushSupported());
  const [pushPermission, setPushPermission] = useState<NotificationPermission | 'unsupported'>(
    () => notificationService.getPermissionStatus()
  );
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  const loadPreferences = useCallback(async () => {
    try {
      const prefs = await notificationService.getPreferences();
      setPreferences(prefs);
      const permission = notificationService.getPermissionStatus();
      setPushPermission(permission);
      setDeviceRegistered(false);
      if (pushSupported && prefs.push_enabled && permission === 'granted') {
        setDeviceRegistered(await notificationService.enablePushNotifications());
      }
    } catch (error) {
      console.error('Failed to load notification preferences:', error);
      setMessage({ type: 'error', text: 'Failed to load notification settings' });
    } finally {
      setIsLoading(false);
    }
  }, [pushSupported]);

  useEffect(() => {
    void loadPreferences();
    const refresh = () => { void loadPreferences(); };
    window.addEventListener('focus', refresh);
    return () => window.removeEventListener('focus', refresh);
  }, [loadPreferences]);

  const handleEnablePush = async () => {
    setIsSaving(true);
    setMessage(null);
    try {
      const success = await notificationService.enablePushNotifications();
      if (success) {
        setPushPermission('granted');
        const updated = await notificationService.updatePreferences({ push_enabled: true });
        setPreferences(updated);
        setDeviceRegistered(true);
        setMessage({ type: 'success', text: 'Push notifications enabled on this device.' });
      } else {
        const permission = notificationService.getPermissionStatus();
        setPushPermission(permission);
        setMessage({
          type: 'error',
          text: permission === 'denied'
            ? 'Notifications are blocked. Allow them in your browser settings, then return here.'
            : 'Registration failed. Please try again.',
        });
      }
    } catch (error) {
      console.error('Failed to enable push:', error);
      setMessage({ type: 'error', text: 'Failed to enable push notifications' });
    } finally {
      setPushPermission(notificationService.getPermissionStatus());
      setIsSaving(false);
    }
  };

  const updatePreference = async (key: keyof NotificationPreferences, value: boolean) => {
    if (!preferences) return;

    setIsSaving(true);
    try {
      const updated = await notificationService.updatePreferences({ [key]: value });
      setPreferences(updated);
    } catch (error) {
      console.error('Failed to update preference:', error);
      const serverError = (error as { response?: { data?: { error?: string } } }).response?.data?.error;
      setMessage({ type: 'error', text: serverError || 'Failed to save setting' });
    } finally {
      setIsSaving(false);
    }
  };

  const handleSendTest = async () => {
    setIsSendingTest(true);
    setMessage(null);
    try {
      const result = await notificationService.sendTestPush();
      setMessage({
        type: 'success',
        text: `Test notification accepted by ${result.accepted_devices} of ${result.attempted_devices} registered ${result.attempted_devices === 1 ? 'device' : 'devices'}.`,
      });
    } catch (error) {
      const serverMessage = (error as { response?: { data?: { message?: string } } }).response?.data?.message;
      setMessage({ type: 'error', text: serverMessage || 'Could not send the test notification.' });
    } finally {
      setIsSendingTest(false);
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-8">
        <Loader2 className="w-6 h-6 animate-spin text-primary-600" />
      </div>
    );
  }

  const pushGlobalEnabled = pushSupported && pushPermission === 'granted' && deviceRegistered && preferences?.push_enabled;

  return (
    <div className="space-y-6">
      {/* Push Notifications Section */}
      <div id="push-notifications" className="scroll-mt-24 bg-white rounded-xl border border-slate-200 p-6">
        <div className="flex items-center gap-3 mb-4">
          {pushGlobalEnabled ? (
            <BellRing className="w-5 h-5 text-primary-600" />
          ) : (
            <BellOff className="w-5 h-5 text-slate-400" />
          )}
          <h3 className="text-lg font-semibold text-slate-900">Push Notifications</h3>
        </div>

        <div role="status" className={`rounded-lg p-3 mb-4 text-sm ${pushGlobalEnabled ? 'bg-emerald-50 text-emerald-800' : 'bg-amber-50 text-amber-900'}`}>
          <p className="font-semibold">
            {pushGlobalEnabled ? 'This device is registered for push notifications' : 'This device still needs push setup'}
          </p>
        </div>

        {!pushSupported ? (
          <p className="text-sm text-slate-500 mb-4">
            Push is unavailable here. On iPhone or iPad, add Rally to your Home Screen and open it from there.
          </p>
        ) : pushPermission === 'denied' ? (
          <div className="bg-amber-50 border border-amber-200 rounded-lg p-4 mb-4">
            <p className="text-sm text-amber-800">
              Notifications are blocked. Allow them in your browser settings, then return to this page.
            </p>
          </div>
        ) : pushPermission !== 'granted' || !deviceRegistered || !preferences?.push_enabled ? (
          <div className="mb-4">
            <p className="text-sm text-slate-600 mb-3">
              Enable this device to receive RSVP deadlines, cancellations and waitlist openings even when Rally is closed.
            </p>
            <button
              onClick={handleEnablePush}
              disabled={isSaving}
              className="bg-primary-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-primary-700 transition-colors disabled:opacity-50 flex items-center gap-2"
            >
              {isSaving ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <Bell className="w-4 h-4" />
              )}
              Enable on this device
            </button>
          </div>
        ) : null}

        {preferences && (
          <div className="flex items-center justify-between mb-4 pb-4 border-b border-slate-200">
            <div>
              <p className="text-sm font-medium text-slate-700">Account push alerts</p>
              <p className="text-xs text-slate-500">Allow alerts on registered devices</p>
            </div>
            <ToggleSwitch
              label="Account push alerts"
              enabled={preferences.push_enabled}
              onChange={(enabled) => enabled ? handleEnablePush() : updatePreference('push_enabled', false)}
              disabled={isSaving}
            />
          </div>
        )}

        {isAdmin && pushGlobalEnabled && (
          <div className="mb-4 rounded-xl border border-primary-200 bg-primary-50/60 p-4">
            <p className="text-sm font-semibold text-slate-900">Test your registered devices</p>
            <p className="mt-1 text-xs leading-5 text-slate-600">
              Sends a one-off test only to devices registered to your account. Preview notifications stay disabled for everyone else.
            </p>
            <button
              type="button"
              onClick={handleSendTest}
              disabled={isSendingTest}
              className="btn-secondary mt-3 gap-2"
            >
              {isSendingTest ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
              Send test notification
            </button>
          </div>
        )}

        {pushGlobalEnabled && preferences && (
          <div className="space-y-1">
            <div className="text-xs font-medium text-slate-500 uppercase tracking-wide mb-2">
              Notification Types
            </div>
            <NotificationRow
              label="Session Reminders"
              description="Get reminded before sessions you've RSVP'd to"
              pushEnabled={preferences.push_session_reminders}
              onPushChange={(enabled) => updatePreference('push_session_reminders', enabled)}
              pushDisabled={isSaving || !pushGlobalEnabled}
            />
            <NotificationRow
              label="RSVP Deadlines"
              description="Get alerted when RSVP deadlines are approaching"
              pushEnabled={preferences.push_rsvp_deadlines}
              onPushChange={(enabled) => updatePreference('push_rsvp_deadlines', enabled)}
              pushDisabled={isSaving || !pushGlobalEnabled}
            />
            <NotificationRow
              label="Waitlist Updates"
              description="Get notified when spots open up"
              pushEnabled={preferences.push_waitlist_updates}
              onPushChange={(enabled) => updatePreference('push_waitlist_updates', enabled)}
              pushDisabled={isSaving || !pushGlobalEnabled}
            />
            <NotificationRow
              label="Club Announcements"
              description="Receive important updates from club admins"
              pushEnabled={preferences.push_admin_announcements}
              onPushChange={(enabled) => updatePreference('push_admin_announcements', enabled)}
              pushDisabled={isSaving || !pushGlobalEnabled}
            />
            <NotificationRow
              label="Balance Alerts"
              description="Hear about it when a session leaves you running low"
              pushEnabled={preferences.push_balance_alerts}
              onPushChange={(enabled) => updatePreference('push_balance_alerts', enabled)}
              pushDisabled={isSaving || !pushGlobalEnabled}
            />
          </div>
        )}
      </div>

      {/* Message */}
      {message && (
        <div className={`p-3 rounded-lg text-sm ${
          message.type === 'success' ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-700'
        }`}>
          {message.text}
        </div>
      )}
    </div>
  );
}
