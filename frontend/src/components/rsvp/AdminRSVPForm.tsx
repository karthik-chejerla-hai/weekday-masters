import { useEffect, useId, useState } from 'react';
import axios from 'axios';
import { api } from '../../services/api';
import type { GuestRSVP, RSVP, SelectableRSVPStatus, User } from '../../types';
import { displayName } from '../../utils/members';

const responseLabels = { in: 'In', maybe: 'Maybe', out: 'Out', waitlisted: 'Waitlisted' };

interface Props {
  sessionId: string;
  rsvps: RSVP[];
  guests?: GuestRSVP[];
  initialGuest?: GuestRSVP;
  onCancelEdit?: () => void;
  allowGuests?: boolean;
  deadlinePassed: boolean;
  isFull: boolean;
  onSaved: () => Promise<void>;
}

export default function AdminRSVPForm({ sessionId, rsvps, guests = [], initialGuest, onCancelEdit, allowGuests = true, deadlinePassed, isFull, onSaved }: Props) {
  const fieldId = useId();
  const [kind, setKind] = useState<'member' | 'guest'>(initialGuest ? 'guest' : 'member');
  const [guestId, setGuestId] = useState(initialGuest?.id || '');
  const [guestName, setGuestName] = useState(initialGuest?.guest_name || '');
  const [members, setMembers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const [userId, setUserId] = useState(initialGuest?.charged_to_user_id || '');
  const [status, setStatus] = useState<SelectableRSVPStatus>(initialGuest?.status || 'in');
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

  const selectGuest = (guest?: GuestRSVP) => {
    setGuestId(guest?.id || ''); setGuestName(guest?.guest_name || '');
    setUserId(guest?.charged_to_user_id || ''); setStatus(guest?.status || 'in');
    setError(''); setSuccess('');
  };
  const normalizedName = (name: string) => name.trim().replace(/\s+/gu, ' ').toLowerCase();
  const duplicateGuest = guests.find(guest => guest.id !== guestId && normalizedName(guest.guest_name) === normalizedName(guestName));
  const selectedGuest = guests.find(guest => guest.id === guestId) || initialGuest;
  const isGuest = kind === 'guest';
  const guestNeedsPlace = isGuest && status === 'in' && selectedGuest?.status !== 'in';
  const current = rsvps.find(rsvp => rsvp.user_id === userId);
  const selectedMember = members.find(member => member.id === userId);

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!selectedMember || saving || (isGuest && (!guestName.trim() || duplicateGuest || !allowGuests || status === 'maybe'))) return;
    setSaving(true);
    setError('');
    setSuccess('');
    try {
      if (isGuest) {
        const saved = await api.saveGuestRSVP(sessionId, {
          guest_name: guestName.trim().replace(/\s+/gu, ' '), charged_to_user_id: userId, status: status as 'in' | 'out',
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
      <h2 className="font-semibold text-slate-900">{initialGuest ? `Edit ${initialGuest.guest_name}’s RSVP` : 'RSVP for a player'}</h2>
      <p className="text-sm text-slate-600">{initialGuest ? 'Change the guest name, paying member, or response.' : 'Record or change a member or guest response as an admin.'}</p>
      {deadlinePassed && <p className="text-sm text-amber-800">RSVP is closed. You can still record this player's response using your admin privileges. The deadline stays unchanged for other members.</p>}
      {loadError ? <p role="alert" className="text-sm text-red-700">Could not load members. Reload this page to try again.</p> : (
        <>
          {allowGuests && !initialGuest && <div>
            <label htmlFor={`${fieldId}-admin-rsvp-kind`} className="block text-sm font-medium text-slate-700 mb-1">Player type</label>
            <select id={`${fieldId}-admin-rsvp-kind`} className="input min-h-11" value={kind} disabled={saving} onChange={event => {
              setKind(event.target.value as 'member' | 'guest');
              setGuestId(''); setGuestName(''); setUserId(''); setStatus('in'); setError(''); setSuccess('');
            }}>
              <option value="member">Member</option>
              <option value="guest">Guest</option>
            </select>
          </div>}
          {isGuest && <>
            {!initialGuest && <div>
              <label htmlFor={`${fieldId}-admin-rsvp-guest`} className="block text-sm font-medium text-slate-700 mb-1">Guest RSVP</label>
              <select id={`${fieldId}-admin-rsvp-guest`} className="input min-h-11" value={guestId} disabled={saving} onChange={event => {
                const guest = guests.find(item => item.id === event.target.value);
                selectGuest(guest);
              }}>
                <option value="">Add a new guest</option>
                {guests.map(guest => <option key={guest.id} value={guest.id}>{guest.guest_name} ({responseLabels[guest.status]}) · {displayName(guest.charged_to_user)}</option>)}
              </select>
            </div>}
            <div>
              <label htmlFor={`${fieldId}-admin-rsvp-guest-name`} className="block text-sm font-medium text-slate-700 mb-1">Guest name</label>
              <input id={`${fieldId}-admin-rsvp-guest-name`} className="input min-h-11" value={guestName} required maxLength={255} disabled={saving}
                onChange={event => { setGuestName(event.target.value); setSuccess(''); }} />
            </div>
          </>}
          <div>
            <label htmlFor={`${fieldId}-admin-rsvp-player`} className="block text-sm font-medium text-slate-700 mb-1">{isGuest ? 'Charged to member' : 'Player'}</label>
            <select id={`${fieldId}-admin-rsvp-player`} className="input min-h-11" value={userId} disabled={loading || saving} required onChange={event => {
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
            <label htmlFor={`${fieldId}-admin-rsvp-response`} className="block text-sm font-medium text-slate-700 mb-1">Response</label>
            <select id={`${fieldId}-admin-rsvp-response`} className="input min-h-11" value={status} disabled={!userId || saving} onChange={event => {
              setStatus(event.target.value as SelectableRSVPStatus);
              setSuccess('');
              setError('');
            }}>
              <option value="in">In</option>
              {!isGuest && <option value="maybe">Maybe</option>}
              <option value="out">Out</option>
            </select>
          </div>
          {isGuest && duplicateGuest && <div role="alert" className="text-sm text-amber-800">
            <p>{duplicateGuest.guest_name} already has an RSVP for this session.</p>
            {!initialGuest && <button type="button" className="min-h-11 underline font-medium" disabled={saving} onClick={() => selectGuest(duplicateGuest)}>Edit existing guest</button>}
          </div>}
          {!isGuest && isFull && status === 'in' && <p className="text-sm text-amber-800">This session is full. Saving In as an admin confirms the player beyond capacity instead of joining the waitlist.</p>}
          {isFull && guestNeedsPlace && <p className="text-sm text-amber-800">This session is full. A guest needs a vacant place before you can save In.</p>}
          <button type="submit" className="btn-primary" disabled={!userId || saving || loading || (isGuest && (!guestName.trim() || !!duplicateGuest || (isFull && guestNeedsPlace)))}>{saving ? 'Saving…' : isGuest ? 'Save guest RSVP' : 'Save player RSVP'}</button>
        </>
      )}
      {onCancelEdit && <button type="button" className="btn-secondary ml-2" disabled={saving} onClick={onCancelEdit}>Cancel</button>}
      {error && <p role="alert" className="text-sm text-red-700">{error}</p>}
      {success && <p role="status" className="text-sm text-green-700">{success}</p>}
    </form>
  );
}
