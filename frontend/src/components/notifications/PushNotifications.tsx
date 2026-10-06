import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../../context/useAuth';
import { notificationService } from '../../services/notifications';

// Refresh existing device tokens after sign-in. Permission prompts stay in settings.
export default function PushNotifications() {
  const { user, isApproved } = useAuth();
  const userID = user?.id;
  const [settingsVersion, setSettingsVersion] = useState(0);
  useEffect(() => {
    const changed = () => setSettingsVersion((version) => version + 1);
    window.addEventListener('notification-settings-changed', changed);
    return () => window.removeEventListener('notification-settings-changed', changed);
  }, []);
  const [notice, setNotice] = useState<{ title: string; body: string; path: string } | null>(null);
  useEffect(() => {
    if (!userID || !isApproved || !notificationService.isPushSupported()) return;
    let cancelled = false;
    let unsubscribe: (() => void) | undefined;
    void (async () => {
      try {
        const prefs = await notificationService.getPreferences();
        if (cancelled || !prefs.push_enabled || notificationService.getPermissionStatus() !== 'granted') return;
        await notificationService.enablePushNotifications();
        if (cancelled) return;
        unsubscribe = notificationService.setupForegroundHandler((title, body, data) => {
          if (!cancelled) setNotice({ title, body, path: data?.session_id
            ? `/sessions/${encodeURIComponent(data.session_id)}`
            : data?.type?.startsWith('balance_') ? '/money' : '/dashboard' });
        });
      } catch (error) {
        console.error('Could not restore push notifications:', error);
      }
    })();
    return () => { cancelled = true; unsubscribe?.(); };
  }, [userID, isApproved, settingsVersion]);
  if (!notice) return null;
  return (
    <div role="status" className="fixed right-4 top-20 z-50 max-w-sm rounded-xl border border-primary-200 bg-white p-4 shadow-lg">
      <Link to={notice.path} onClick={() => setNotice(null)}>
        <strong className="block text-slate-900">{notice.title}</strong>
        <span className="text-sm text-slate-600">{notice.body}</span>
      </Link>
      <button type="button" className="mt-2 block text-sm text-primary-700" onClick={() => setNotice(null)}>Dismiss</button>
    </div>
  );
}
