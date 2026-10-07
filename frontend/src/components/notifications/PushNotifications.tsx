import { useEffect, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useAuth } from '../../context/useAuth';
import { notificationService } from '../../services/notifications';

// Refresh existing tokens after sign-in. Ask for browser permission only after a setup click.
export default function PushNotifications() {
  const { user, isApproved } = useAuth();
  const userID = user?.id;
  const { pathname } = useLocation();
  const [setup, setSetup] = useState<{ userID: string; needed: boolean; message: string } | null>(null);
  const [dismissedFor, setDismissedFor] = useState<string | null>(null);
  const [settingsVersion, setSettingsVersion] = useState(0);
  useEffect(() => {
    const changed = () => setSettingsVersion((version) => version + 1);
    window.addEventListener('notification-settings-changed', changed);
    window.addEventListener('focus', changed);
    return () => {
      window.removeEventListener('notification-settings-changed', changed);
      window.removeEventListener('focus', changed);
    };
  }, []);
  const [notice, setNotice] = useState<{ title: string; body: string; path: string } | null>(null);
  useEffect(() => {
    if (!userID || !isApproved) return;
    let cancelled = false;
    let unsubscribe: (() => void) | undefined;
    void (async () => {
      try {
        const prefs = await notificationService.getPreferences();
        if (cancelled) return;
        if (!prefs.push_enabled) {
          setSetup({ userID, needed: false, message: '' });
          return;
        }
        const permission = notificationService.getPermissionStatus();
        if (!notificationService.isPushSupported() || permission !== 'granted') {
          setSetup({ userID, needed: true, message: permission === 'denied'
            ? 'Notifications are blocked on this device. Open settings for help.'
            : 'Your account allows push alerts, but this device still needs setup.' });
          return;
        }
        const registered = await notificationService.enablePushNotifications();
        if (cancelled) return;
        setSetup({ userID, needed: !registered, message: 'We could not register this device. Open settings to try again.' });
        if (!registered) return;
        unsubscribe = notificationService.setupForegroundHandler((title, body, data) => {
          if (!cancelled) setNotice({ title, body, path: data?.session_id
            ? `/sessions/${encodeURIComponent(data.session_id)}`
            : data?.type?.startsWith('balance_') ? '/money' : '/dashboard' });
        });
      } catch (error) {
        console.error('Could not restore push notifications:', error);
        if (!cancelled) setSetup({ userID, needed: true, message: 'We could not check push setup on this device. Open settings to try again.' });
      }
    })();
    return () => { cancelled = true; unsubscribe?.(); };
  }, [userID, isApproved, settingsVersion]);
  const showSetup = isApproved && userID && setup?.userID === userID && setup.needed && dismissedFor !== userID && pathname !== '/profile';
  return (
    <>
    {showSetup && (
      <section aria-label="Push notification setup" className="border-b border-amber-200 bg-amber-50 px-4 py-3 sm:px-6 lg:px-10">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-3">
          <div>
            <p className="font-semibold text-amber-950">Push alerts are not ready on this device</p>
            <p className="text-sm text-amber-900">{setup.message}</p>
          </div>
          <div className="flex items-center gap-3">
            <Link to="/profile#push-notifications" className="rounded-lg bg-amber-900 px-4 py-2 text-sm font-semibold text-white">Set up push alerts</Link>
            <button type="button" onClick={() => setDismissedFor(userID)} className="px-2 py-2 text-sm text-amber-900">Not now</button>
          </div>
        </div>
      </section>
    )}
    {notice && (
    <div role="status" className="fixed right-4 top-20 z-50 max-w-sm rounded-xl border border-primary-200 bg-white p-4 shadow-lg">
      <Link to={notice.path} onClick={() => setNotice(null)}>
        <strong className="block text-slate-900">{notice.title}</strong>
        <span className="text-sm text-slate-600">{notice.body}</span>
      </Link>
      <button type="button" className="mt-2 block text-sm text-primary-700" onClick={() => setNotice(null)}>Dismiss</button>
    </div>
    )}
    </>
  );
}
