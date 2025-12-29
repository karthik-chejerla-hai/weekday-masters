import { useState, useEffect } from 'react';
import { Bell, Mail, Loader2, BellOff, BellRing, Smartphone } from 'lucide-react';
import { notificationService, NotificationPreferences } from '../../services/notifications';

interface ToggleSwitchProps {
  enabled: boolean;
  onChange: (enabled: boolean) => void;
  disabled?: boolean;
}

function ToggleSwitch({ enabled, onChange, disabled }: ToggleSwitchProps) {
  return (
    <button
      type="button"
      role="switch"
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
  emailEnabled: boolean;
  onPushChange: (enabled: boolean) => void;
  onEmailChange: (enabled: boolean) => void;
  pushDisabled?: boolean;
  emailDisabled?: boolean;
}

function NotificationRow({
  label,
  description,
  pushEnabled,
  emailEnabled,
  onPushChange,
  onEmailChange,
  pushDisabled,
  emailDisabled
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
            enabled={pushEnabled}
            onChange={onPushChange}
            disabled={pushDisabled}
          />
        </div>
        <div className="flex items-center gap-2">
          <Mail className="w-4 h-4 text-slate-400" />
          <ToggleSwitch
            enabled={emailEnabled}
            onChange={onEmailChange}
            disabled={emailDisabled}
          />
        </div>
      </div>
    </div>
  );
}

export default function NotificationSettings() {
  const [preferences, setPreferences] = useState<NotificationPreferences | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [pushSupported] = useState(() => notificationService.isPushSupported());
  const [pushPermission, setPushPermission] = useState<NotificationPermission | 'unsupported'>(
    () => notificationService.getPermissionStatus()
  );
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  useEffect(() => {
    loadPreferences();
  }, []);

  const loadPreferences = async () => {
    try {
      const prefs = await notificationService.getPreferences();
      setPreferences(prefs);
    } catch (error) {
      console.error('Failed to load notification preferences:', error);
      setMessage({ type: 'error', text: 'Failed to load notification settings' });
    } finally {
      setIsLoading(false);
    }
  };

  const handleEnablePush = async () => {
    setIsSaving(true);
    setMessage(null);
    try {
      const success = await notificationService.enablePushNotifications();
      if (success) {
        setPushPermission('granted');
        await updatePreference('push_enabled', true);
        setMessage({ type: 'success', text: 'Push notifications enabled!' });
      } else {
        setMessage({ type: 'error', text: 'Failed to enable push notifications. Please check your browser settings.' });
      }
    } catch (error) {
      console.error('Failed to enable push:', error);
      setMessage({ type: 'error', text: 'Failed to enable push notifications' });
    } finally {
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
      setMessage({ type: 'error', text: 'Failed to save setting' });
    } finally {
      setIsSaving(false);
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-8">
        <Loader2 className="w-6 h-6 animate-spin text-primary-600" />
      </div>
    );
  }

  const pushGlobalEnabled = pushSupported && pushPermission === 'granted' && preferences?.push_enabled;
  const emailGlobalEnabled = preferences?.email_enabled ?? true;

  return (
    <div className="space-y-6">
      {/* Push Notifications Section */}
      <div className="bg-white rounded-xl border border-slate-200 p-6">
        <div className="flex items-center gap-3 mb-4">
          {pushGlobalEnabled ? (
            <BellRing className="w-5 h-5 text-primary-600" />
          ) : (
            <BellOff className="w-5 h-5 text-slate-400" />
          )}
          <h3 className="text-lg font-semibold text-slate-900">Push Notifications</h3>
        </div>

        {!pushSupported ? (
          <p className="text-sm text-slate-500 mb-4">
            Push notifications are not supported in your browser.
          </p>
        ) : pushPermission === 'denied' ? (
          <div className="bg-amber-50 border border-amber-200 rounded-lg p-4 mb-4">
            <p className="text-sm text-amber-800">
              Push notifications are blocked. Please enable them in your browser settings to receive notifications.
            </p>
          </div>
        ) : pushPermission !== 'granted' ? (
          <div className="mb-4">
            <p className="text-sm text-slate-600 mb-3">
              Enable push notifications to receive instant updates about sessions and RSVPs.
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
              Enable Push Notifications
            </button>
          </div>
        ) : (
          <div className="flex items-center justify-between mb-4 pb-4 border-b border-slate-200">
            <div>
              <p className="text-sm font-medium text-slate-700">Push Notifications</p>
              <p className="text-xs text-slate-500">Receive notifications on this device</p>
            </div>
            <ToggleSwitch
              enabled={preferences?.push_enabled ?? false}
              onChange={(enabled) => updatePreference('push_enabled', enabled)}
              disabled={isSaving}
            />
          </div>
        )}

        {pushPermission === 'granted' && preferences?.push_enabled && (
          <div className="space-y-1">
            <div className="text-xs font-medium text-slate-500 uppercase tracking-wide mb-2">
              Notification Types
            </div>
            <NotificationRow
              label="Session Reminders"
              description="Get reminded before sessions you've RSVP'd to"
              pushEnabled={preferences.push_session_reminders}
              emailEnabled={preferences.email_session_reminders}
              onPushChange={(enabled) => updatePreference('push_session_reminders', enabled)}
              onEmailChange={(enabled) => updatePreference('email_session_reminders', enabled)}
              pushDisabled={isSaving || !pushGlobalEnabled}
              emailDisabled={isSaving || !emailGlobalEnabled}
            />
            <NotificationRow
              label="RSVP Deadlines"
              description="Get alerted when RSVP deadlines are approaching"
              pushEnabled={preferences.push_rsvp_deadlines}
              emailEnabled={preferences.email_rsvp_deadlines}
              onPushChange={(enabled) => updatePreference('push_rsvp_deadlines', enabled)}
              onEmailChange={(enabled) => updatePreference('email_rsvp_deadlines', enabled)}
              pushDisabled={isSaving || !pushGlobalEnabled}
              emailDisabled={isSaving || !emailGlobalEnabled}
            />
            <NotificationRow
              label="Waitlist Updates"
              description="Get notified when spots open up"
              pushEnabled={preferences.push_waitlist_updates}
              emailEnabled={preferences.email_waitlist_updates}
              onPushChange={(enabled) => updatePreference('push_waitlist_updates', enabled)}
              onEmailChange={(enabled) => updatePreference('email_waitlist_updates', enabled)}
              pushDisabled={isSaving || !pushGlobalEnabled}
              emailDisabled={isSaving || !emailGlobalEnabled}
            />
            <NotificationRow
              label="Club Announcements"
              description="Receive important updates from club admins"
              pushEnabled={preferences.push_admin_announcements}
              emailEnabled={preferences.email_admin_announcements}
              onPushChange={(enabled) => updatePreference('push_admin_announcements', enabled)}
              onEmailChange={(enabled) => updatePreference('email_admin_announcements', enabled)}
              pushDisabled={isSaving || !pushGlobalEnabled}
              emailDisabled={isSaving || !emailGlobalEnabled}
            />
          </div>
        )}
      </div>

      {/* Email Notifications Section (shown when push not enabled) */}
      {(!pushSupported || pushPermission !== 'granted' || !preferences?.push_enabled) && preferences && (
        <div className="bg-white rounded-xl border border-slate-200 p-6">
          <div className="flex items-center gap-3 mb-4">
            <Mail className="w-5 h-5 text-primary-600" />
            <h3 className="text-lg font-semibold text-slate-900">Email Notifications</h3>
          </div>

          <div className="flex items-center justify-between mb-4 pb-4 border-b border-slate-200">
            <div>
              <p className="text-sm font-medium text-slate-700">Email Notifications</p>
              <p className="text-xs text-slate-500">Receive notifications via email</p>
            </div>
            <ToggleSwitch
              enabled={preferences.email_enabled}
              onChange={(enabled) => updatePreference('email_enabled', enabled)}
              disabled={isSaving}
            />
          </div>

          {preferences.email_enabled && (
            <div className="space-y-3">
              {[
                { key: 'email_session_reminders' as const, label: 'Session Reminders', desc: 'Get reminded before sessions' },
                { key: 'email_rsvp_deadlines' as const, label: 'RSVP Deadlines', desc: 'Get deadline alerts' },
                { key: 'email_waitlist_updates' as const, label: 'Waitlist Updates', desc: 'Get notified when spots open' },
                { key: 'email_admin_announcements' as const, label: 'Club Announcements', desc: 'Receive club updates' }
              ].map(({ key, label, desc }) => (
                <div key={key} className="flex items-center justify-between py-2">
                  <div>
                    <p className="text-sm font-medium text-slate-700">{label}</p>
                    <p className="text-xs text-slate-500">{desc}</p>
                  </div>
                  <ToggleSwitch
                    enabled={preferences[key]}
                    onChange={(enabled) => updatePreference(key, enabled)}
                    disabled={isSaving}
                  />
                </div>
              ))}
            </div>
          )}
        </div>
      )}

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
