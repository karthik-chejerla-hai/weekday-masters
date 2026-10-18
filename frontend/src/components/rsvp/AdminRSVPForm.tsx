import { useEffect, useState } from 'react';
import axios from 'axios';
import { api } from '../../services/api';
import type { RSVP, SelectableRSVPStatus, User } from '../../types';
import { displayName } from '../../utils/members';

const responseLabels = { in: 'In', maybe: 'Maybe', out: 'Out', waitlisted: 'Waitlisted' };

interface Props {
  sessionId: string;
  rsvps: RSVP[];
  deadlinePassed: boolean;
  isFull: boolean;
  onSaved: () => Promise<void>;
}

export default function AdminRSVPForm({ sessionId, rsvps, deadlinePassed, isFull, onSaved }: Props) {
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

  const current = rsvps.find(rsvp => rsvp.user_id === userId);
  const selectedMember = members.find(member => member.id === userId);

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!selectedMember || saving) return;
    setSaving(true);
    setError('');
    setSuccess('');
    try {
      await api.adminAddRSVP(sessionId, userId, status);
      await onSaved();
      setSuccess(`${displayName(selectedMember)}'s RSVP saved as ${responseLabels[status]}.`);
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
      <p className="text-sm text-slate-600">Record or change a member's response as an admin.</p>
      {deadlinePassed && <p className="text-sm text-amber-800">RSVP is closed. You can still record this player's response using your admin privileges. The deadline stays unchanged for other members.</p>}
      {loadError ? <p role="alert" className="text-sm text-red-700">Could not load members. Reload this page to try again.</p> : (
        <>
          <div>
            <label htmlFor="admin-rsvp-player" className="block text-sm font-medium text-slate-700 mb-1">Player</label>
            <select id="admin-rsvp-player" className="input-field" value={userId} disabled={loading || saving} required onChange={event => {
              const id = event.target.value;
              setUserId(id);
              const response = rsvps.find(rsvp => rsvp.user_id === id)?.status;
              setStatus(response && response !== 'waitlisted' ? response : 'in');
              setError('');
              setSuccess('');
            }}>
              <option value="">{loading ? 'Loading members…' : 'Select a player'}</option>
              {members.map(member => <option key={member.id} value={member.id}>{displayName(member)}{members.some(other => other.id !== member.id && displayName(other) === displayName(member)) ? ` (${member.name})` : ''}</option>)}
            </select>
            {userId && <p className="mt-2 text-sm text-slate-600">Current response: {current ? responseLabels[current.status] : 'Not responded'}</p>}
          </div>
          <div>
            <label htmlFor="admin-rsvp-response" className="block text-sm font-medium text-slate-700 mb-1">Response</label>
            <select id="admin-rsvp-response" className="input-field" value={status} disabled={!userId || saving} onChange={event => {
              setStatus(event.target.value as SelectableRSVPStatus);
              setSuccess('');
              setError('');
            }}>
              <option value="in">In</option>
              <option value="maybe">Maybe</option>
              <option value="out">Out</option>
            </select>
          </div>
          {isFull && status === 'in' && <p className="text-sm text-amber-800">This session is full. Saving In as an admin confirms the player beyond capacity instead of joining the waitlist.</p>}
          <button type="submit" className="btn-primary" disabled={!userId || saving || loading}>{saving ? 'Saving…' : 'Save player RSVP'}</button>
        </>
      )}
      {error && <p role="alert" className="text-sm text-red-700">{error}</p>}
      {success && <p role="status" className="text-sm text-green-700">{success}</p>}
    </form>
  );
}
