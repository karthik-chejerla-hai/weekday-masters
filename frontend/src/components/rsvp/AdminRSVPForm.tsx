import { useEffect, useState } from 'react';
import axios from 'axios';
import { api } from '../../services/api';
import type { GuestRSVP, RSVP, SelectableRSVPStatus, User } from '../../types';
import { displayName } from '../../utils/members';

const responseLabels = { in: 'In', maybe: 'Maybe', out: 'Out', waitlisted: 'Waitlisted' };

interface Props {
  sessionId: string;
  rsvps: RSVP[];
  guests?: GuestRSVP[];
  allowGuests?: boolean;
  deadlinePassed: boolean;
  isFull: boolean;
  onSaved: () => Promise<void>;
}

export default function AdminRSVPForm({ sessionId, rsvps, guests = [], allowGuests = true, deadlinePassed, isFull, onSaved }: Props) {
  const [kind, setKind] = useState<'member' | 'guest'>('member');
  const [guestId, setGuestId] = useState('');
  const [guestName, setGuestName] = useState('');
  const [members, setMembers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const [userId, setUserId] = useState('');
  const [status, setStatus] = useState<SelectableRSVPStatus>('in');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  useEffect(() => {
    let active = true;
    api.listMembers().then(data => {
      if (active) setMembers(data.filter(member => member.membership_status === 'approved')
        .sort((a, b) => displayName(a).localeCompare(displayName(b))));
    }).catch(() => {
      if (active) setLoadError(true);
    }).finally(() => {
      if (active) setLoading(false);
    });
    return () => { active = false; };
  }, []);

  const selectedGuest = guests.find(guest => guest.id === guestId);
  const isGuest = kind === 'guest';
  const guestNeedsPlace = isGuest && status === 'in' && selectedGuest?.status !== 'in';
  const current = rsvps.find(rsvp => rsvp.user_id === userId);
  const selectedMember = members.find(member => member.id === userId);

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!selectedMember || saving || (isGuest && (!guestName.trim() || !allowGuests || status === 'maybe'))) return;
    setSaving(true);
    setError('');
    setSuccess('');
    try {
      if (isGuest) {
        const saved = await api.saveGuestRSVP(sessionId, {
          guest_name: guestName.trim(), charged_to_user_id: userId, status: status as 'in' | 'out',
        }, guestId || undefined);
        setGuestId(saved.id);
      } else {
        await api.adminAddRSVP(sessionId, userId, status);
      }
      await onSaved();
      setSuccess(`${isGuest ? guestName.trim() : displayName(selectedMember)}'s RSVP saved as ${responseLabels[status]}.`);
    } catch (err) {
      setError(axios.isAxiosError(err) && typeof err.response?.data?.error === 'string'
        ? err.response.data.error : 'Could not save this RSVP. Please try again.');
    } finally {
      setSaving(false);
    }
  };

  return (
    <form onSubmit={save} className="bg-white rounded-xl border border-slate-200 p-6 space-y-4">
      <h2 className="font-semibold text-slate-900">RSVP for a player</h2>
      <p className="text-sm text-slate-600">Record or change a member or guest response as an admin.</p>
      {deadlinePassed && <p className="text-sm text-amber-800">RSVP is closed. You can still record this player's response using your admin privileges. The deadline stays unchanged for other members.</p>}
      {loadError ? <p role="alert" className="text-sm text-red-700">Could not load members. Reload this page to try again.</p> : (
        <>
          {allowGuests && <div>
            <label htmlFor="admin-rsvp-kind" className="block text-sm font-medium text-slate-700 mb-1">Player type</label>
            <select id="admin-rsvp-kind" className="input min-h-11" value={kind} disabled={saving} onChange={event => {
              setKind(event.target.value as 'member' | 'guest');
              setGuestId(''); setGuestName(''); setUserId(''); setStatus('in'); setError(''); setSuccess('');
            }}>
              <option value="member">Member</option>
              <option value="guest">Guest</option>
            </select>
          </div>}
          {isGuest && <>
            <div>
              <label htmlFor="admin-rsvp-guest" className="block text-sm font-medium text-slate-700 mb-1">Guest RSVP</label>
              <select id="admin-rsvp-guest" className="input min-h-11" value={guestId} disabled={saving} onChange={event => {
                const guest = guests.find(item => item.id === event.target.value);
                setGuestId(guest?.id || ''); setGuestName(guest?.guest_name || '');
                setUserId(guest?.charged_to_user_id || ''); setStatus(guest?.status || 'in'); setError(''); setSuccess('');
              }}>
                <option value="">Add a new guest</option>
                {guests.map(guest => <option key={guest.id} value={guest.id}>{guest.guest_name} ({responseLabels[guest.status]}) · {displayName(guest.charged_to_user)}</option>)}
              </select>
            </div>
            <div>
              <label htmlFor="admin-rsvp-guest-name" className="block text-sm font-medium text-slate-700 mb-1">Guest name</label>
              <input id="admin-rsvp-guest-name" className="input min-h-11" value={guestName} required maxLength={255} disabled={saving}
                onChange={event => { setGuestName(event.target.value); setSuccess(''); }} />
            </div>
          </>}
          <div>
            <label htmlFor="admin-rsvp-player" className="block text-sm font-medium text-slate-700 mb-1">{isGuest ? 'Charged to member' : 'Player'}</label>
            <select id="admin-rsvp-player" className="input min-h-11" value={userId} disabled={loading || saving} required onChange={event => {
              const id = event.target.value;
              setUserId(id);
              const response = rsvps.find(rsvp => rsvp.user_id === id)?.status;
              if (!isGuest) setStatus(response && response !== 'waitlisted' ? response : 'in');
              setError('');
              setSuccess('');
            }}>
              <option value="">{loading ? 'Loading members…' : isGuest ? 'Select a paying member' : 'Select a player'}</option>
              {members.map(member => <option key={member.id} value={member.id}>{displayName(member)}{members.some(other => other.id !== member.id && displayName(other) === displayName(member)) ? ` (${member.name})` : ''}</option>)}
            </select>
            {!isGuest && userId && <p className="mt-2 text-sm text-slate-600">Current response: {current ? responseLabels[current.status] : 'Not responded'}</p>}
          </div>
          <div>
            <label htmlFor="admin-rsvp-response" className="block text-sm font-medium text-slate-700 mb-1">Response</label>
            <select id="admin-rsvp-response" className="input min-h-11" value={status} disabled={!userId || saving} onChange={event => {
              setStatus(event.target.value as SelectableRSVPStatus);
              setSuccess('');
              setError('');
            }}>
              <option value="in">In</option>
              {!isGuest && <option value="maybe">Maybe</option>}
              <option value="out">Out</option>
            </select>
          </div>
          {!isGuest && isFull && status === 'in' && <p className="text-sm text-amber-800">This session is full. Saving In as an admin confirms the player beyond capacity instead of joining the waitlist.</p>}
          {isFull && guestNeedsPlace && <p className="text-sm text-amber-800">This session is full. A guest needs a vacant place before you can save In.</p>}
          <button type="submit" className="btn-primary" disabled={!userId || saving || loading || (isGuest && (!guestName.trim() || (isFull && guestNeedsPlace)))}>{saving ? 'Saving…' : isGuest ? 'Save guest RSVP' : 'Save player RSVP'}</button>
        </>
      )}
      {error && <p role="alert" className="text-sm text-red-700">{error}</p>}
      {success && <p role="status" className="text-sm text-green-700">{success}</p>}
    </form>
  );
}
