import { useState } from 'react';
import { api, type MemberPushStatus } from '../../services/api';
import type { User } from '../../types';
import { displayName } from '../../utils/members';

const alerts = [
  ['push_session_reminders', 'Session reminders'],
  ['push_rsvp_deadlines', 'RSVP deadlines'],
  ['push_waitlist_updates', 'Waitlist updates'],
  ['push_admin_announcements', 'Admin announcements'],
  ['push_balance_alerts', 'Balance alerts'],
] as const;

function registrationDate(value: string) {
  return new Intl.DateTimeFormat('en-AU', {
    dateStyle: 'medium', timeStyle: 'short', timeZone: 'Australia/Sydney',
  }).format(new Date(value));
}

export default function MemberPushPanel({ member }: { member: User }) {
  const [open, setOpen] = useState(false);
  const [status, setStatus] = useState<MemberPushStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);

  async function load() {
    setLoading(true);
    setError(false);
    setStatus(null);
    try {
      setStatus(await api.getMemberPushStatus(member.id));
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }

  const prefs = status?.preferences;
  return (
    <section className="mt-3 border-t border-slate-100 pt-3" aria-label={`Push alerts for ${displayName(member)}`}>
      <button
        type="button"
        aria-expanded={open}
        aria-controls={`push-${member.id}`}
        className="text-sm font-medium text-primary-700 hover:underline"
        onClick={() => {
          setOpen(!open);
          if (!open && !loading) void load();
        }}
      >
        {open ? 'Hide push alerts and devices' : 'Push alerts and devices'}
      </button>
      {open && (
        <div id={`push-${member.id}`} className="mt-3 rounded-lg bg-slate-50 p-4 text-sm space-y-4">
          {loading && <p role="status">Loading push settings…</p>}
          {error && <div role="alert">Could not load push settings. <button type="button" className="text-primary-700 underline" onClick={() => void load()}>Try again</button></div>}
          {status && <>
            <div className="flex flex-wrap items-center gap-2">
              <strong>{prefs ? (prefs.push_enabled ? 'Account push: On' : 'Account push: Off') : 'No saved push settings'}</strong>
              <span className="rounded-full bg-white border border-slate-200 px-2 py-1">{status.devices.length} registered {status.devices.length === 1 ? 'device' : 'devices'}</span>
              <button type="button" className="text-primary-700 underline ml-auto" onClick={() => void load()}>Refresh</button>
            </div>
            {member.membership_status !== 'approved' && <p>This member is not approved. These are their saved settings.</p>}
            {prefs ? <div>
              <h3 className="font-medium mb-2">Saved alert choices</h3>
              {!prefs.push_enabled && <p className="text-slate-600 mb-2">Account push is off. These choices apply when the member turns it on.</p>}
              <dl className="grid sm:grid-cols-2 gap-x-6 gap-y-2">
                {alerts.map(([key, label]) => <div key={key} className="flex justify-between gap-3"><dt>{label}</dt><dd className="font-medium">{prefs[key] ? 'On' : 'Off'}</dd></div>)}
              </dl>
            </div> : <p className="text-slate-600">The app uses default alert settings until a preference record exists. Defaults do not confirm permission.</p>}
            <div>
              <h3 className="font-medium mb-2">Registered devices</h3>
              {status.devices.length === 0 ? <p>No registered devices. Push alerts cannot reach this member.</p> :
                <ul className="space-y-2">{status.devices.map((device, index) => <li key={device.id} className="rounded-lg border border-slate-200 bg-white p-3">
                  <p className="font-medium">{device.device_name.trim() || `Unnamed browser ${index + 1}`}</p>
                  <p className="text-slate-600">First registered: {registrationDate(device.created_at)}</p>
                  <p className="text-slate-600">Last registered: {registrationDate(device.last_registered_at)}</p>
                </li>)}</ul>}
            </div>
            <p className="text-xs text-slate-500">Times use Sydney time. Registration records past browser permission. It does not confirm current phone or browser settings, or successful delivery. Global notification pauses can also stop alerts.</p>
          </>}
        </div>
      )}
    </section>
  );
}
