import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { CheckCircle2, Loader2 } from 'lucide-react';
import { formatInTimeZone } from 'date-fns-tz';
import { api } from '../services/api';
import type { PlayerBalance, SharedExpenseInput, SharedExpensePreview, SplitMethod } from '../types';
import { formatCents, parseDollarsToCents } from '../components/money/format';
import { assistantError, assistantErrorCode } from '../components/assistant/errors';

const BANK = 'bank';
const METHODS: { value: SplitMethod; label: string; hint: string }[] = [
  { value: 'equal', label: 'Equally', hint: 'Everyone selected pays the same share.' },
  { value: 'exact', label: 'Exact amounts', hint: 'Enter what each person owes. The amounts must add up to the total.' },
  { value: 'shares', label: 'By shares', hint: 'Give each person a number of shares, such as 2 for someone who had twice as much.' },
];
const MONEY = /^\d+(\.\d{1,2})?$/;

/** Cents for a well-formed dollar amount, otherwise null. */
function cents(value: string): number | null {
  const trimmed = value.trim();
  return MONEY.test(trimmed) ? parseDollarsToCents(trimmed) : null;
}

/** Records a bill that is not a session, such as a meal or a coffee. */
export default function AdminSharedExpense() {
  const today = formatInTimeZone(new Date(), 'Australia/Sydney', 'yyyy-MM-dd');
  // The server refuses a date more than 365 days back. Calendar arithmetic in UTC avoids a zone shift.
  const earliest = new Date(Date.parse(`${today}T00:00:00Z`) - 365 * 86_400_000).toISOString().slice(0, 10);
  const [members, setMembers] = useState<PlayerBalance[]>([]);
  const [loading, setLoading] = useState(true);
  const [description, setDescription] = useState('');
  const [date, setDate] = useState(today);
  const [total, setTotal] = useState('');
  const [payer, setPayer] = useState(BANK);
  const [method, setMethod] = useState<SplitMethod>('equal');
  const [selected, setSelected] = useState<string[]>([]);
  const [amounts, setAmounts] = useState<Record<string, string>>({});
  const [weights, setWeights] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [saving, setSaving] = useState(false);
  // Kept apart from the preview, so nothing that clears the preview can hide a saved expense.
  const [saved, setSaved] = useState<{ id: string; description: string; totalCents: number } | null>(null);
  const [reversing, setReversing] = useState(false);
  const [reversed, setReversed] = useState(false);
  const [error, setError] = useState('');
  const [preview, setPreview] = useState<SharedExpensePreview | null>(null);
  const generation = useRef(0);
  const review = useRef<HTMLElement>(null);
  const confirming = useRef(false);
  // The payer this form ticked on its own, so a corrected payer can be unticked again.
  const autoPayer = useRef<string | null>(null);
  // One ID per reviewed bill, keyed by what the bill says. A retry of the same
  // contents reuses it, so the server answers with the first result instead of
  // charging twice, even after an Edit that changed nothing. Only different
  // contents start a new one.
  const request = useRef({ contents: '', id: '' });
  const requestIdFor = (input: SharedExpenseInput) => {
    const contents = JSON.stringify(input);
    if (request.current.contents !== contents) request.current = { contents, id: crypto.randomUUID() };
    return request.current.id;
  };
  // Any edit makes the figures on screen stale, so the preview goes with it.
  const invalidate = () => {
    if (confirming.current) return;
    generation.current++; setPreview(null); setError('');
  };

  useEffect(() => {
    let active = true;
    api.listBalances()
      .then((balances) => { if (active) setMembers(balances); })
      .catch((err) => { if (active) setError(assistantError(err, 'Could not load members.')); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, []);

  // On a phone the preview lands below the fold, so bring it to the admin.
  useEffect(() => {
    if (preview) review.current?.scrollIntoView?.({ behavior: 'smooth', block: 'start' });
  }, [preview]);

  const totalCents = cents(total);
  const chosen = members.filter((member) => selected.includes(member.user_id));
  const entered = chosen.reduce((sum, member) => sum + (cents(amounts[member.user_id] ?? '') ?? 0), 0);
  const remaining = totalCents === null ? null : totalCents - entered;

  const choosePayer = (value: string) => {
    invalidate();
    setPayer(value);
    // The person who paid usually ate too. Added once, here, so clearing them sticks.
    // A payer picked by mistake is unticked again unless the admin ticked them too.
    let next = autoPayer.current && autoPayer.current !== value ? selected.filter((id) => id !== autoPayer.current) : selected;
    autoPayer.current = null;
    if (value !== BANK && !next.includes(value)) { next = [...next, value]; autoPayer.current = value; }
    setSelected(next);
  };

  const build = (): SharedExpenseInput | string => {
    if (!description.trim()) return 'Enter a description, such as "Dinner after the session".';
    if (totalCents === null || totalCents <= 0) return 'Enter the total in dollars and cents, such as 84.50.';
    if (!chosen.length) return 'Select at least one member to split with.';
    const participants = [];
    for (const member of chosen) {
      if (method === 'exact') {
        const amount = cents(amounts[member.user_id] ?? '');
        if (amount === null || amount <= 0) return `Enter an amount for ${member.name}.`;
        participants.push({ user_id: member.user_id, amount_cents: amount });
      } else if (method === 'shares') {
        const weight = Number(weights[member.user_id] ?? '1');
        if (!Number.isInteger(weight) || weight < 1 || weight > 100) return `Enter a whole number of shares from 1 to 100 for ${member.name}.`;
        participants.push({ user_id: member.user_id, weight });
      } else {
        participants.push({ user_id: member.user_id });
      }
    }
    if (method === 'exact' && remaining !== 0) return 'The amounts must add up to the total.';
    return { description: description.trim(), occurred_on: date, total_cents: totalCents, payer_user_id: payer === BANK ? null : payer, split_method: method, participants };
  };

  const makePreview = async () => {
    if (busy) return;
    const input = build();
    if (typeof input === 'string') { setError(input); return; }
    const current = ++generation.current;
    setBusy(true); setError(''); setPreview(null);
    try {
      const result = await api.previewSharedExpense(input);
      if (current === generation.current) setPreview(result);
    } catch (err) {
      if (current === generation.current) setError(assistantError(err, 'Could not prepare the expense.'));
    } finally { setBusy(false); }
  };

  const confirm = async () => {
    if (!preview || confirming.current) return;
    confirming.current = true; setSaving(true); setError('');
    try {
      const transaction = await api.confirmSharedExpense({ ...preview.input, request_id: requestIdFor(preview.input), expected_preview: preview.fingerprint });
      setSaved({ id: transaction.id, description: preview.input.description, totalCents: preview.input.total_cents });
      window.dispatchEvent(new Event('rally:balances-changed'));
    } catch (err) {
      // The preview stays, so "Confirm" again resends the same request ID. If the
      // first attempt did reach the server, that returns its result; nobody pays twice.
      const stale = assistantErrorCode(err) === 'preview_stale';
      if (stale) setPreview(null);
      setError(assistantError(err, stale ? 'The expense changed. Preview it again.' : 'Could not save the expense. Check your connection, then confirm again.'));
    } finally { confirming.current = false; setSaving(false); }
  };

  // The ledger is append-only: a mistake is undone by a reversing transaction.
  const reverse = async () => {
    if (reversing || !saved) return;
    setReversing(true); setError('');
    try {
      await api.reverseTransaction(saved.id, `Reversal of ${saved.description}`);
      setReversed(true);
      window.dispatchEvent(new Event('rally:balances-changed'));
    } catch (err) {
      setError(assistantError(err, 'Could not reverse the expense. Try again.'));
    } finally { setReversing(false); }
  };

  if (loading) return <div role="status" className="card p-8 flex justify-center"><Loader2 className="h-6 w-6 animate-spin" /><span className="sr-only">Loading expense form</span></div>;

  if (saved) return <div className="mx-auto max-w-2xl space-y-4">
    <div role="status" className="rounded-2xl border border-emerald-200 bg-emerald-50 p-5 space-y-3">
      <p className="flex items-center gap-2 font-semibold text-emerald-900"><CheckCircle2 className="h-5 w-5" /> {reversed ? 'Expense reversed' : 'Expense recorded'}</p>
      <p className="text-sm text-emerald-900">{saved.description} · {formatCents(saved.totalCents)}. {reversed ? 'Member balances are back where they were. Record the expense again if it is still owed.' : 'Member balances are updated.'}</p>
      <div className="flex flex-wrap gap-3"><Link className="btn-primary" to="/money?tab=ledger">See the ledger</Link><Link className="btn-outline" to="/money">Back to Money</Link></div>
    </div>
    {error && <div role="alert" className="rounded-xl bg-red-50 p-3 text-sm text-red-800">{error}</div>}
    {!reversed && <p className="text-sm text-slate-600">Made a mistake? <button type="button" className="underline" disabled={reversing} onClick={() => void reverse()}>{reversing ? 'Reversing…' : 'Reverse this expense'}</button>, then record it again.</p>}
  </div>;

  const hint = METHODS.find((option) => option.value === method)?.hint;

  return <div className="mx-auto max-w-2xl space-y-5">
    <div className="page-heading"><p className="page-kicker">Off the court</p><h1 className="page-title">Record shared expense</h1>
      <p className="mt-2 text-sm text-slate-600">Split a meal, a coffee or another bill that is not a session.</p>
    </div>
    <form className="card p-5" onSubmit={(event) => { event.preventDefault(); void makePreview(); }}>
      {/* Locked while saving: an edit then would discard the preview that is being posted. */}
      <fieldset disabled={saving} className="space-y-5">
      <div><label className="label" htmlFor="shared-description">Description</label>
        <input id="shared-description" className="input" type="text" maxLength={200} required value={description} onChange={(event) => { invalidate(); setDescription(event.target.value); }} placeholder="Dinner after the session" /></div>
      <div className="grid gap-4 sm:grid-cols-2">
        <div><label className="label" htmlFor="shared-total">Total ($)</label>
          <input id="shared-total" className="input" type="text" inputMode="decimal" required value={total} onChange={(event) => { invalidate(); setTotal(event.target.value); }} placeholder="0.00" /></div>
        <div><label className="label" htmlFor="shared-date">Date</label>
          <input id="shared-date" className="input" type="date" min={earliest} max={today} required value={date} onChange={(event) => { invalidate(); setDate(event.target.value); }} /></div>
      </div>
      <div className="grid gap-4 sm:grid-cols-2">
        <div><label className="label" htmlFor="shared-payer">Paid by</label>
          <select id="shared-payer" className="input" value={payer} onChange={(event) => choosePayer(event.target.value)}>
            <option value={BANK}>Club bank</option>
            {members.map((member) => <option key={member.user_id} value={member.user_id}>{member.name}</option>)}
          </select></div>
        <div><label className="label" htmlFor="shared-method">Split</label>
          <select id="shared-method" className="input" value={method} onChange={(event) => { invalidate(); setMethod(event.target.value as SplitMethod); }}>
            {METHODS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
          </select></div>
      </div>
      <fieldset><legend className="text-sm font-semibold">Who shares the bill · {chosen.length} selected</legend>
        <p className="mt-1 mb-3 text-sm text-slate-500">{hint}</p>
        <div className="grid gap-2 sm:grid-cols-2">{members.map((member) => {
          const checked = selected.includes(member.user_id);
          return <div key={member.user_id} className="flex min-h-12 items-center gap-3 rounded-xl border border-slate-200 p-3 text-sm">
            <label className="flex min-w-0 flex-1 items-center gap-3"><input type="checkbox" className="h-4 w-4 accent-cyan-700" checked={checked} onChange={(event) => {
              const next = event.target.checked;
              invalidate();
              if (member.user_id === autoPayer.current) autoPayer.current = null;
              setSelected((current) => next ? [...current, member.user_id] : current.filter((value) => value !== member.user_id));
            }} /><span className="truncate">{member.name}</span></label>
            {checked && method === 'exact' && <input aria-label={`Amount for ${member.name}`} className="input w-24 text-right" type="text" inputMode="decimal" placeholder="0.00" value={amounts[member.user_id] ?? ''} onChange={(event) => { invalidate(); setAmounts((current) => ({ ...current, [member.user_id]: event.target.value })); }} />}
            {checked && method === 'shares' && <input aria-label={`Shares for ${member.name}`} className="input w-20 text-right" type="number" inputMode="numeric" min="1" max="100" step="1" value={weights[member.user_id] ?? '1'} onChange={(event) => { invalidate(); setWeights((current) => ({ ...current, [member.user_id]: event.target.value })); }} />}
          </div>;
        })}</div>
        {method === 'exact' && remaining !== null && <p className={`mt-3 text-sm tabular-nums ${remaining === 0 ? 'text-emerald-700' : 'text-amber-800'}`} aria-live="polite">
          {remaining === 0 ? 'The amounts add up to the total.' : remaining > 0 ? `${formatCents(remaining)} still to assign.` : `${formatCents(-remaining)} over the total.`}
        </p>}
      </fieldset>
      {error && <div role="alert" className="rounded-xl bg-red-50 p-3 text-sm text-red-800">{error}</div>}
      <button type="submit" className="btn-primary w-full gap-2" disabled={busy}>{busy && <Loader2 className="h-4 w-4 animate-spin" />}{busy ? 'Preparing preview…' : 'Preview expense'}</button>
      </fieldset>
    </form>
    {preview && <section ref={review} aria-label="Expense preview" className="card overflow-hidden scroll-mt-24">
      <div className="border-b border-slate-200 bg-primary-50/60 p-5">
        <p className="page-kicker">Review before saving</p>
        <h2 className="mt-1 text-lg font-semibold text-slate-950">{preview.input.description}</h2>
        <p className="mt-1 text-sm text-slate-600">{formatInTimeZone(`${preview.input.occurred_on}T12:00:00Z`, 'UTC', 'EEEE, d MMM yyyy')} · Paid by {preview.payer_name}</p>
      </div>
      <div className="space-y-5 p-5">
        <div className="flex items-baseline justify-between border-b border-slate-100 pb-3"><span className="font-medium">Total expense</span><strong className="text-2xl tabular-nums">{formatCents(preview.input.total_cents)}</strong></div>
        <div><h3 className="mb-2 text-sm font-semibold">Shares · {preview.lines.filter((line) => line.share_cents > 0).length} members</h3>
          <ul className="divide-y divide-slate-100">{preview.lines.filter((line) => line.share_cents > 0).map((line) => <li key={line.user_id} className="flex justify-between gap-4 py-2 text-sm"><span>{line.name}</span><strong className="tabular-nums">{formatCents(line.share_cents)}</strong></li>)}</ul>
          {preview.input.split_method !== 'exact' && <p className="mt-2 text-xs text-slate-500">A one-cent difference can occur when the total does not divide exactly.</p>}
        </div>
        <div className="rounded-xl bg-slate-50 p-3 text-sm">
          {preview.input.payer_user_id === null
            ? <p>The club bank goes down by <strong>{formatCents(preview.input.total_cents)}</strong>. Each member is charged their share.</p>
            : preview.lines.filter((line) => line.user_id === preview.input.payer_user_id).map((line) => <p key={line.user_id}>
              {line.name} is credited <strong>{formatCents(line.net_cents)}</strong>{line.share_cents > 0 ? <>: the {formatCents(preview.input.total_cents)} paid, less their own {formatCents(line.share_cents)} share</> : null}. The club bank does not change.
            </p>)}
        </div>
        <div className="flex flex-wrap gap-3">
          <button type="button" className="btn-primary gap-2" disabled={saving} onClick={() => void confirm()}>{saving && <Loader2 className="h-4 w-4 animate-spin" />}{saving ? 'Saving…' : 'Confirm expense'}</button>
          <button type="button" className="btn-outline" disabled={saving} onClick={() => { invalidate(); document.getElementById('shared-description')?.focus(); }}>Edit</button>
        </div>
      </div>
    </section>}
    <p className="text-xs text-slate-500"><Link className="underline" to="/money">Back to Money</Link></p>
  </div>;
}
