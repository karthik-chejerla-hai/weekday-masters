import { isDeviceSetupStorageChange } from '../../services/push-device';
import { useState, useEffect, useCallback } from 'react';
import { Bell, Loader2, Send } from 'lucide-react';
import { notificationService } from '../../services/notifications';
import { useAuth } from '../../context/useAuth';

export default function NotificationSettings({ isAdmin = false }: { isAdmin?: boolean }) {
  const { user } = useAuth();
  const userID = user?.id;
  const [registered, setRegistered] = useState(false);
  const [loading, setLoading] = useState(true);
  const [checking, setChecking] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const supported = notificationService.isPushSupported();
  const permission = notificationService.getPermissionStatus();
  const refresh = useCallback(async (manual = false) => {
    if (!userID) return;
    setChecking(true);
    if (manual) setMessage(null);
    try {
      setRegistered(await notificationService.restorePushNotifications(userID));
      setError(null);
      if (manual) setMessage('Device check complete.');
    }
    catch { setError('Could not check this device. Try again.'); }
    finally { setLoading(false); setChecking(false); }
  }, [userID]);
  useEffect(() => {
    void refresh(); const changed = () => { void refresh(); };
    const storageChanged = (event: StorageEvent) => { if (isDeviceSetupStorageChange(event)) changed(); };
    window.addEventListener('focus', changed); window.addEventListener('storage', storageChanged);
    return () => { window.removeEventListener('focus', changed); window.removeEventListener('storage', storageChanged); };
  }, [refresh]);
  async function change(enable: boolean) {
    if (!userID) return;
    setBusy(true); setError(null); setMessage(null);
    try {
      if (enable) {
        const success = await notificationService.enablePushNotifications(userID);
        setRegistered(success);
        if (!success) { setError('Allow notifications in your browser settings, then try again.'); return; }
      } else {
        await notificationService.disablePushNotifications(userID); setRegistered(false);
      }
      window.dispatchEvent(new Event('notification-settings-changed'));
    } catch { setError('Device setup could not finish. Try again; Rally has not confirmed success.'); }
    finally { setBusy(false); }
  }
  async function test() {
    setBusy(true); setError(null);
    try { const result = await notificationService.sendTestPush(); setMessage(`Test notification accepted by ${result.accepted_devices} of ${result.attempted_devices} registered devices.`); }
    catch { setError('Could not send the test notification.'); }
    finally { setBusy(false); }
  }
  return <section id="push-notifications" className="card p-6 space-y-4">
    <h3 className="text-lg font-semibold flex items-center gap-2"><Bell className="w-5 h-5" />Push notifications</h3>
    <p className="text-sm text-slate-600">Push includes RSVP reminders, session charges, balance alerts and admin announcements. All supported push alerts are included.</p>
    {loading || checking ? <p role="status">Checking this device…</p> : <p role="status">{registered ? 'Push set up on this device' : 'Push is not set up on this device'}</p>}
    {!supported && <p>Push is unavailable here. On iPhone or iPad, add Rally to your Home Screen first.</p>}
    {permission === 'denied' && <p>Notifications are blocked. Allow them in your browser settings, then return here.</p>}
    {!loading && <div className="flex flex-wrap gap-3">
      <button className="btn-primary" disabled={busy || checking || (!registered && (!supported || permission === 'denied'))} onClick={() => { void change(!registered); }}>{busy && <Loader2 className="h-4 w-4 animate-spin" />}{registered ? 'Disable on this device' : 'Enable on this device'}</button>
      <button className="btn-secondary" disabled={busy || checking} onClick={() => { void refresh(true); }}>{checking && <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />}{checking ? 'Checking…' : 'Check again'}</button>
      {isAdmin && registered && <button className="btn-secondary" disabled={busy || checking} onClick={() => { void test(); }}><Send className="h-4 w-4" />Send test notification</button>}
    </div>}
    {error && <p role="alert" className="text-red-700">{error}</p>}{message && <p role="status">{message}</p>}
  </section>;
}
