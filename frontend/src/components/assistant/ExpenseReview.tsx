import { useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { CheckCircle2, Loader2 } from 'lucide-react';
import { formatInTimeZone } from 'date-fns-tz';
import { api } from '../../services/api';
import type { ExpensePreview } from '../../types';
import { formatCents } from '../money/format';
import { assistantError, assistantErrorCode } from './errors';

export default function ExpenseReview({ preview, onRefresh, onEdit }: { preview: ExpensePreview; onRefresh: (next: ExpensePreview) => void; onEdit?: () => void }) {
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState('');
  const [blocked, setBlocked] = useState(false);
  const locked = useRef(false);
  const { settlement, input, session } = preview;
  const mixedAttendance = input.total_hours === 3 && settlement.lines.some((line) => !line.in_extra);
  const confirm = async () => {
    if (locked.current || saved || blocked) return;
    locked.current = true; setSaving(true); setError('');
    try {
      await api.confirmExpense(session.id, { ...input, expected_preview: preview.fingerprint });
      setSaved(true);
      window.dispatchEvent(new Event('rally:balances-changed'));
    } catch (err) {
      setError(assistantError(err, 'Could not save the expense. Try again.'));
      if (assistantErrorCode(err) === 'preview_changed') {
        setBlocked(true);
        try { const next = await api.previewExpense(session.id, input); onRefresh(next); setBlocked(false); }
        catch (refreshError) { setError(assistantError(refreshError, 'Could not refresh the preview. Open the form to try again.')); }
      }
      if (assistantErrorCode(err) === 'session_already_settled') setBlocked(true);
    } finally { locked.current = false; setSaving(false); }
  };

  if (saved) return <div role="status" className="rounded-2xl border border-emerald-200 bg-emerald-50 p-5 space-y-3">
    <p className="flex items-center gap-2 font-semibold text-emerald-900"><CheckCircle2 className="h-5 w-5" /> Expense recorded</p>
    <p className="text-sm text-emerald-900">Player balances, court credit and shuttle stock are updated.</p>
    <div className="flex flex-wrap gap-3"><Link className="btn-primary" to={`/sessions/${session.id}/settlement`}>See the split</Link><Link className="btn-outline" to="/dashboard">Back to Home</Link></div>
  </div>;

  return <section aria-label="Expense preview" className="card overflow-hidden">
    <div className="border-b border-slate-200 bg-primary-50/60 p-5">
      <p className="page-kicker">Review before saving</p>
      <h2 className="mt-1 text-lg font-semibold text-slate-950">{session.title}</h2>
      <p className="mt-1 text-sm text-slate-600">{formatInTimeZone(session.starts_at, 'Australia/Sydney', 'EEEE, d MMM yyyy')} · {input.total_hours} hours · {input.shuttles_used} shuttles</p>
    </div>
    <div className="space-y-5 p-5">
      <dl className="grid grid-cols-2 gap-3 text-sm">
        <div><dt className="text-slate-500">Court cost</dt><dd className="mt-1 font-semibold tabular-nums">{formatCents(settlement.totals.court_cents)}</dd></div>
        <div><dt className="text-slate-500">Shuttle cost</dt><dd className="mt-1 font-semibold tabular-nums">{formatCents(settlement.totals.shuttle_cents)}</dd></div>
      </dl>
      <div className="flex items-baseline justify-between border-y border-slate-100 py-3"><span className="font-medium">Total expense</span><strong className="text-2xl tabular-nums">{formatCents(settlement.totals.charged_cents)}</strong></div>
      <div><h3 className="mb-2 text-sm font-semibold">{mixedAttendance ? 'Player charges' : 'Equal shares'} · {settlement.lines.length} players</h3>
        {mixedAttendance && <p className="mb-2 text-sm text-slate-500">{settlement.lines.length} players share the first two hours. {settlement.lines.filter((line) => line.in_extra).length} share the extra hour. Shuttle cost is split by time: two thirds for the first two hours and one third for the extra hour.</p>}
        <ul className="divide-y divide-slate-100">{settlement.lines.map((line) => <li key={line.user_id} className="flex justify-between gap-4 py-2 text-sm"><span>{line.name}{mixedAttendance && <span className="ml-2 text-xs text-slate-500">{line.in_extra ? '3 hours' : '2 hours only'}</span>}</span><strong className="tabular-nums">{formatCents(line.amount_cents)}</strong></li>)}</ul>
        <p className="mt-2 text-xs text-slate-500">{mixedAttendance ? 'Each time period is split to the nearest cent. All player charges add up to the total expense.' : 'A one-cent difference can occur when the total does not divide exactly.'}</p>
      </div>
      <div className="rounded-xl bg-slate-50 p-3 text-sm space-y-1"><p>After this expense: <strong>{settlement.stock_after.units} shuttles</strong> and <strong>{formatCents(preview.court_credit_after_cents)} court credit</strong>.</p>
        {preview.court_topup_needed && <p className="text-amber-800">Top up court credit before the next standard two-hour booking. It needs {formatCents(preview.next_court_cost_cents)}.</p>}
      </div>
      {error && <p role="alert" className="rounded-lg bg-amber-50 p-3 text-sm text-amber-900">{error}</p>}
      <button type="button" className="btn-primary w-full gap-2" onClick={confirm} disabled={saving || blocked}>{saving && <Loader2 className="h-4 w-4 animate-spin" />}{saving ? 'Saving expense…' : `Confirm expense · ${formatCents(settlement.totals.charged_cents)}`}</button>
      {onEdit ? <button type="button" className="btn-outline w-full" disabled={saving} onClick={onEdit}>Edit details</button> : <Link className="btn-outline w-full" to={`/admin/sessions/${session.id}/expense`} state={{ draft: input }}>Edit in form</Link>}
    </div>
  </section>;
}
