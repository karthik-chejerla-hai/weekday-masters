import { useEffect, useRef, useState } from 'react';
import { Link, useLocation, useParams } from 'react-router-dom';
import { Loader2, Mic } from 'lucide-react';
import { formatInTimeZone } from 'date-fns-tz';
import { api } from '../services/api';
import type { ExpenseInput, ExpensePreview, PlayerBalance, Session } from '../types';
import ExpenseReview from '../components/assistant/ExpenseReview';
import { assistantError } from '../components/assistant/errors';

export default function AdminExpense() {
  const { id } = useParams<{ id: string }>();
  const location = useLocation();
  const draft = (location.state as { draft?: ExpenseInput } | null)?.draft;
  const [session, setSession] = useState<Session | null>(null);
  const [members, setMembers] = useState<PlayerBalance[]>([]);
  const [selected, setSelected] = useState<string[]>([]);
  const [extraSelected, setExtraSelected] = useState<string[]>([]);
  const [hours, setHours] = useState(2);
  const [shuttles, setShuttles] = useState('');
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [preview, setPreview] = useState<ExpensePreview | null>(null);
  const generation = useRef(0);
  const invalidate = () => { generation.current++; setPreview(null); setError(''); };

  useEffect(() => {
    let active = true;
    const cancelPreview = () => { generation.current++; };
    if (!id) return;
    setLoading(true); setPreview(null);
    Promise.all([api.getSession(id), api.listBalances()]).then(([data, balances]) => {
      if (!active) return;
      setSession(data.session); setMembers(balances);
      const participants = draft?.participant_ids ?? data.session.rsvps?.filter((rsvp) => rsvp.status === 'in').map((rsvp) => rsvp.user_id) ?? [];
      setSelected(participants);
      setExtraSelected(draft?.extra_participant_ids ?? participants);
      setHours(draft?.total_hours ?? 2); setShuttles(draft ? String(draft.shuttles_used) : '');
    }).catch((err) => { if (active) setError(assistantError(err, 'Could not open this session.')); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; cancelPreview(); };
  }, [id, draft]);

  const makePreview = async () => {
    if (!id || busy) return;
    const count = Number(shuttles);
    if (shuttles.trim() === '' || !Number.isInteger(count) || count < 0 || count > 200 || !selected.length) { setError('Enter a whole shuttle count from 0 to 200 and select at least one player.'); return; }
    if (hours === 3 && !extraSelected.length) { setError('Select at least one player for the extra hour, or choose 2 hours.'); return; }
    const current = ++generation.current;
    setBusy(true); setError(''); setPreview(null);
    try {
      const result = await api.previewExpense(id, { total_hours: hours, shuttles_used: count, participant_ids: selected, ...(hours === 3 ? { extra_participant_ids: extraSelected } : {}) });
      if (current === generation.current) setPreview(result);
    } catch (err) { if (current === generation.current) setError(assistantError(err, 'Could not prepare the expense.')); }
    finally { setBusy(false); }
  };
  if (loading) return <div role="status" className="card p-8 flex justify-center"><Loader2 className="h-6 w-6 animate-spin" /><span className="sr-only">Loading expense form</span></div>;

  return <div className="mx-auto max-w-2xl space-y-5">
    <div className="page-heading"><p className="page-kicker">After the game</p><h1 className="page-title">Record session expense</h1>
      {session && <p className="mt-2 text-sm text-slate-600">{session.title} · {formatInTimeZone(session.starts_at || session.session_date, 'Australia/Sydney', 'd MMM yyyy')}</p>}
    </div>
    <Link className="btn-secondary gap-2" to={`/assistant?session=${id}`}><Mic className="h-4 w-4" /> Use voice or text</Link>
    <form className="card p-5 space-y-5" onSubmit={(event) => { event.preventDefault(); void makePreview(); }}>
      <div className="grid gap-4 sm:grid-cols-2">
        <div><label className="label" htmlFor="expense-hours">Hours played</label><select id="expense-hours" className="input" value={hours} onChange={(event) => { invalidate(); setHours(Number(event.target.value)); }}><option value={2}>2 hours</option><option value={3}>3 hours</option></select></div>
        <div><label className="label" htmlFor="expense-shuttles">Shuttles used</label><input id="expense-shuttles" className="input" type="number" inputMode="numeric" min="0" max="200" step="1" required value={shuttles} onChange={(event) => { invalidate(); setShuttles(event.target.value); }} placeholder="Actual count" /></div>
      </div>
      <fieldset><legend className="text-sm font-semibold">Who pays · {selected.length} selected</legend><p className="mt-1 mb-3 text-sm text-slate-500">Confirmed RSVPs start selected, including no-shows. Change the list as needed. These players share the first two hours.</p>
        <div className="grid gap-2 sm:grid-cols-2">{members.map((member) => <label key={member.user_id} className="flex min-h-12 items-center gap-3 rounded-xl border border-slate-200 p-3 text-sm"><input type="checkbox" className="h-4 w-4 accent-cyan-700" checked={selected.includes(member.user_id)} onChange={(event) => {
          const checked = event.target.checked;
          invalidate();
          setSelected((current) => checked ? [...current, member.user_id] : current.filter((value) => value !== member.user_id));
          setExtraSelected((current) => checked ? [...current, member.user_id] : current.filter((value) => value !== member.user_id));
        }} /><span>{member.name}</span></label>)}</div>
      </fieldset>
      {hours === 3 && <fieldset><legend className="text-sm font-semibold">Who stayed for the extra hour · {extraSelected.length} selected</legend>
        <p className="mt-1 mb-3 text-sm text-slate-500">Clear anyone who left after two hours. Shuttle cost is split by time: two thirds for the first two hours and one third for the extra hour.</p>
        <div className="grid gap-2 sm:grid-cols-2">{members.filter((member) => selected.includes(member.user_id)).map((member) => <label key={member.user_id} className="flex min-h-12 items-center gap-3 rounded-xl border border-slate-200 p-3 text-sm"><input type="checkbox" aria-label={`${member.name}, extra hour`} className="h-4 w-4 accent-cyan-700" checked={extraSelected.includes(member.user_id)} onChange={(event) => {
          const checked = event.target.checked;
          invalidate();
          setExtraSelected((current) => checked ? [...current, member.user_id] : current.filter((value) => value !== member.user_id));
        }} /><span>{member.name}</span></label>)}</div>
      </fieldset>}
      {error && <div role="alert" className="rounded-xl bg-red-50 p-3 text-sm text-red-800">{error}<p className="mt-2"><Link to="/money?tab=assets" className="underline">Check club assets</Link></p></div>}
      <button type="submit" className="btn-primary w-full gap-2" disabled={busy || !session || !selected.length}>{busy && <Loader2 className="h-4 w-4 animate-spin" />}{busy ? 'Preparing preview…' : 'Preview expense'}</button>
    </form>
    {preview && <ExpenseReview preview={preview} onRefresh={setPreview} onEdit={() => { invalidate(); document.getElementById('expense-hours')?.focus(); }} />}
    <p className="text-xs text-slate-500">Need guest charges or waived fees? <Link className="underline" to={`/admin/sessions/${id}/settle`}>Open the advanced settlement form</Link>.</p>
  </div>;
}
