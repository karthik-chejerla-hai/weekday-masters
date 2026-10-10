import { useEffect, useState } from 'react';
import { BellRing, Loader2, X } from 'lucide-react';
import { Link, useLocation } from 'react-router-dom';
import { useAuth } from '../../context/useAuth';
import { notificationService } from '../../services/notifications';

const DISMISSAL_TTL_MS = 7 * 24 * 60 * 60 * 1000;

type SetupState = {
  userID: string;
  kind: 'enable' | 'repair' | 'blocked';
  message: string;
};

function dismissalKey(userID: string) {
  return `rally:push-prompt-dismissed:${userID}`;
}

function wasDismissedRecently(userID: string) {
  try {
    const dismissedAt = Number(window.localStorage.getItem(dismissalKey(userID)));
    return Number.isFinite(dismissedAt) && dismissedAt > 0 && Date.now() - dismissedAt < DISMISSAL_TTL_MS;
  } catch {
    return false;
  }
}

function clearDismissal(userID: string) {
  try {
    window.localStorage.removeItem(dismissalKey(userID));
  } catch {
    // Push setup still works when browser storage is unavailable.
  }
}

function rememberDismissal(userID: string) {
  try {
    window.localStorage.setItem(dismissalKey(userID), String(Date.now()));
  } catch {
    // Keep the in-memory dismissal for this visit.
  }
}

// Restore registered devices after sign-in. Browser permission is requested only
// from the explicit dashboard action or the button in notification settings.
export default function PushNotifications() {
  const { user, isApproved } = useAuth();
  const userID = user?.id;
  const { pathname } = useLocation();
  const [setup, setSetup] = useState<SetupState | null>(null);
  const [dismissed, setDismissed] = useState(false);
  const [isEnabling, setIsEnabling] = useState(false);
  const [enabledMessage, setEnabledMessage] = useState(false);
  const [settingsVersion, setSettingsVersion] = useState(0);
  const [notice, setNotice] = useState<{ title: string; body: string; path: string } | null>(null);

  useEffect(() => {
    setDismissed(userID ? wasDismissedRecently(userID) : false);
  }, [userID]);

  useEffect(() => {
    const changed = () => setSettingsVersion((version) => version + 1);
    window.addEventListener('notification-settings-changed', changed);
    window.addEventListener('focus', changed);
    return () => {
      window.removeEventListener('notification-settings-changed', changed);
      window.removeEventListener('focus', changed);
    };
  }, []);

  useEffect(() => {
    if (!userID || !isApproved) {
      setSetup(null);
      return;
    }
    let cancelled = false;
    let unsubscribe: (() => void) | undefined;
    void (async () => {
      try {
        const preferences = await notificationService.getPreferences();
        if (cancelled) return;

        const supported = notificationService.isPushSupported();
        const permission = notificationService.getPermissionStatus();
        if (supported && permission === 'default') {
          setSetup({
            userID,
            kind: 'enable',
            message: 'Get RSVP deadlines, cancellations and waitlist openings even when Rally is closed.',
          });
          return;
        }
        if (!preferences.push_enabled) {
          setSetup(null);
          return;
        }
        if (!supported || permission === 'denied') {
          setSetup({
            userID,
            kind: 'blocked',
            message: supported
              ? 'Notifications are blocked on this device. Open your settings for help.'
              : 'Push is unavailable here. On iPhone or iPad, add Rally to your Home Screen first.',
          });
          return;
        }

        const registered = await notificationService.enablePushNotifications();
        if (cancelled) return;
        if (!registered) {
          setSetup({
            userID,
            kind: 'repair',
            message: 'Rally could not register this device. Open notification settings to try again.',
          });
          return;
        }

        setSetup(null);
        unsubscribe = notificationService.setupForegroundHandler((title, body, data) => {
          if (!cancelled) setNotice({
            title,
            body,
            path: data?.session_id
              ? `/sessions/${encodeURIComponent(data.session_id)}`
              : data?.type?.startsWith('balance_') ? '/money' : '/dashboard',
          });
        });
      } catch (error) {
        console.error('Could not restore push notifications:', error);
        if (!cancelled) setSetup({
          userID,
          kind: 'repair',
          message: 'Rally could not check this device. Open notification settings to try again.',
        });
      }
    })();
    return () => { cancelled = true; unsubscribe?.(); };
  }, [userID, isApproved, settingsVersion]);

  const enablePush = async () => {
    if (!userID) return;
    setIsEnabling(true);
    setEnabledMessage(false);
    try {
      const registered = await notificationService.enablePushNotifications();
      if (!registered) {
        const permission = notificationService.getPermissionStatus();
        setSetup({
          userID,
          kind: permission === 'denied' ? 'blocked' : 'repair',
          message: permission === 'denied'
            ? 'Notifications are blocked on this device. Open your settings for help.'
            : 'Rally could not register this device. Try again from notification settings.',
        });
        return;
      }
      await notificationService.updatePreferences({ push_enabled: true });
      clearDismissal(userID);
      setDismissed(false);
      setSetup(null);
      setEnabledMessage(true);
    } catch (error) {
      console.error('Could not enable push notifications:', error);
      setSetup({
        userID,
        kind: 'repair',
        message: 'This device connected, but Rally could not save your setting. Open notification settings to retry.',
      });
    } finally {
      setIsEnabling(false);
    }
  };

  const dismissSetup = () => {
    if (!userID) return;
    rememberDismissal(userID);
    setDismissed(true);
  };

  const visibleSetup = pathname === '/dashboard'
    && setup
    && setup.userID === userID
    && (setup.kind !== 'enable' || !dismissed)
    ? setup
    : null;

  return (
    <>
      {visibleSetup && (
        <section aria-label="Push notification setup" className="border-b border-primary-200 bg-primary-50 px-4 py-3 sm:px-6 lg:px-10">
          <div className="mx-auto grid max-w-6xl grid-cols-[2.5rem_minmax(0,1fr)] items-start gap-x-3 gap-y-3 sm:grid-cols-[2.5rem_minmax(0,1fr)_auto] sm:items-center">
            <span className="col-start-1 row-start-1 flex h-10 w-10 items-center justify-center rounded-full bg-primary-100 text-primary-800">
              <BellRing className="h-5 w-5" aria-hidden="true" />
            </span>
            <div className="col-start-2 row-start-1 min-w-0">
              <p className="font-semibold text-slate-950">
                {visibleSetup.kind === 'enable' ? 'Stay in the Rally loop' : 'Push alerts need attention'}
              </p>
              <p className="text-sm text-slate-700">{visibleSetup.message}</p>
            </div>
            <div className="col-span-2 row-start-2 flex items-center gap-2 sm:col-span-1 sm:col-start-3 sm:row-start-1">
              {visibleSetup.kind === 'enable' ? (
                <button type="button" onClick={() => void enablePush()} disabled={isEnabling} className="btn-primary min-h-10 flex-1 text-sm disabled:opacity-60 sm:flex-none">
                  {isEnabling ? <><Loader2 className="h-4 w-4 animate-spin" /> Enabling…</> : 'Enable on this device'}
                </button>
              ) : (
                <Link to="/profile#push-notifications" className="btn-primary min-h-10 flex-1 text-sm sm:flex-none">Notification settings</Link>
              )}
              {visibleSetup.kind === 'enable' && (
                <button type="button" onClick={dismissSetup} className="min-h-10 rounded-lg px-3 text-sm font-medium text-slate-600 hover:bg-primary-100">
                  Not now
                </button>
              )}
            </div>
          </div>
        </section>
      )}

      {enabledMessage && (
        <div role="status" className="fixed right-4 top-20 z-50 flex max-w-sm items-start gap-3 rounded-xl border border-emerald-200 bg-white p-4 shadow-lg">
          <BellRing className="mt-0.5 h-5 w-5 shrink-0 text-emerald-600" aria-hidden="true" />
          <div className="flex-1">
            <strong className="block text-slate-900">Push notifications enabled</strong>
            <span className="text-sm text-slate-600">This device is ready for Rally alerts.</span>
          </div>
          <button type="button" aria-label="Dismiss confirmation" onClick={() => setEnabledMessage(false)} className="rounded p-1 text-slate-500 hover:bg-slate-100">
            <X className="h-4 w-4" aria-hidden="true" />
          </button>
        </div>
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
