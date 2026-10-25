import { useEffect, useState } from 'react';
import { isAxiosError } from 'axios';
import { formatInTimeZone } from 'date-fns-tz';
import { api } from '../../services/api';
import type { AssetPurchase } from '../../types';
import { formatCents } from './format';
import FormActionRow from './FormActionRow';

export default function AssetPurchaseHistory(props: { revision: number; onRecorded: () => void }) {
  const [open, setOpen] = useState(false);
  return <details className="card p-4" onToggle={event => setOpen(event.currentTarget.open)}>
    <summary className="cursor-pointer py-2 font-semibold text-slate-900">Purchase history and corrections</summary>
    {open && <PurchaseHistory {...props} />}
  </details>;
}

function PurchaseHistory({ revision, onRecorded }: { revision: number; onRecorded: () => void }) {
  const [items, setItems] = useState<AssetPurchase[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [retry, setRetry] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [editing, setEditing] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  useEffect(() => {
    let active = true;
    setLoading(true); setError(false);
    api.listAssetPurchases(20, page * 20).then(result => {
      if (active) { setItems(result.items); setTotal(result.total); }
    }).catch(() => { if (active) setError(true); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [revision, page, retry]);
  return <section aria-label="Asset purchase history" className="mt-3 space-y-4">
    <p className="text-sm text-slate-600">Correct the amount or count on a recorded purchase. For a verified bank or court balance, use “Audit bank balance” above and select the asset.</p>
    {success && <p role="status" className="text-sm text-green-800">Purchase corrected. The original, reversal and replacement are retained.</p>}
    {loading && <p role="status">Loading purchases…</p>}
    {error && <p role="alert">Could not load purchases. <button className="underline" onClick={() => setRetry(value => value + 1)}>Try again</button></p>}
    {!loading && !error && items.length === 0 && <p className="text-sm text-slate-500">No purchases on this page.</p>}
    {!loading && !error && items.map(item => <article key={item.id} className="rounded-lg border border-slate-200 p-4 space-y-2">
      <h3 className="font-semibold">{item.kind === 'shuttle_purchase' ? 'Shuttle purchase' : 'Court credit purchase'} · {formatCents(item.amount_cents)}</h3>
      <p className="text-sm text-slate-600">{formatInTimeZone(item.occurred_at, 'Australia/Sydney', 'd MMM yyyy')}{item.kind === 'shuttle_purchase' ? ` · ${item.units} shuttles` : ''}</p>
      <p className="break-words text-sm">{item.description}</p>
      <p className="text-xs text-slate-500">Recorded by {item.actor_name} · {formatInTimeZone(item.created_at, 'Australia/Sydney', 'd MMM yyyy, h:mm a')} (Sydney)</p>
      {item.corrects_transaction_id && <p className="break-words text-sm text-slate-600">Correction of {item.corrects_transaction_id}. Reason: {item.correction_reason}</p>}
      {item.reversed ? <p className="text-sm font-medium text-slate-500">Reversed / corrected</p> : editing === item.id ?
        <PurchaseCorrectionForm key={item.id} purchase={item} onCancel={() => setEditing(null)} onRecorded={() => {
          setEditing(null); setSuccess(true); setRetry(value => value + 1); onRecorded();
        }} /> : <button type="button" className="btn-outline" disabled={editing !== null} onClick={() => { setSuccess(false); setEditing(item.id); }}>Correct purchase</button>}
    </article>)}
    <div className="flex gap-3">
      {page > 0 && <button className="btn-secondary" disabled={loading || editing !== null} onClick={() => setPage(value => value - 1)}>Newer purchases</button>}
      {(page + 1) * 20 < total && <button className="btn-secondary" disabled={loading || editing !== null} onClick={() => setPage(value => value + 1)}>Older purchases</button>}
    </div>
  </section>;
}

export function PurchaseCorrectionForm({ purchase, onRecorded, onCancel }: { purchase: AssetPurchase; onRecorded: () => void; onCancel: () => void }) {
  const [amount, setAmount] = useState(formatCents(purchase.amount_cents).replace(/[$,]/g, ''));
  const [units, setUnits] = useState(String(purchase.units));
  const [reason, setReason] = useState('');
  const [review, setReview] = useState<{ amount_cents: number; units: number; reason: string } | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const isShuttle = purchase.kind === 'shuttle_purchase';
  const prepare = (event: React.FormEvent) => {
    event.preventDefault(); setError(null);
    const match = /^(\d+)(?:\.(\d{1,2}))?$/.exec(amount.trim());
    const cents = match ? Number(match[1]) * 100 + Number((match[2] ?? '').padEnd(2, '0')) : NaN;
    const count = isShuttle ? Number(units) : 0;
    if (!Number.isSafeInteger(cents) || cents <= 0 || cents > 4503599627370495) { setError('Enter a positive amount with up to two decimal places.'); return; }
    if (isShuttle && (!Number.isInteger(count) || count <= 0 || count > 2147483647)) { setError('Enter a positive whole shuttle count.'); return; }
    if (cents === purchase.amount_cents && count === purchase.units) { setError('Change the amount or shuttle count to correct this purchase.'); return; }
    if (!reason.trim()) { setError('Enter a reason for the correction.'); return; }
    setReview({ amount_cents: cents, units: count, reason: reason.trim() });
  };
  const save = async () => {
    if (!review || saving) return;
    setSaving(true); setError(null);
    try { await api.correctAssetPurchase(purchase.id, review); onRecorded(); }
    catch (err) { setError(isAxiosError<{ message?: string }>(err) ? err.response?.data?.message ?? 'Could not save. You can safely retry.' : 'Could not save. You can safely retry.'); }
    finally { setSaving(false); }
  };
  return <div className="border-t border-slate-200 pt-4 space-y-3">
    {error && <p role="alert" className="text-sm text-red-700">{error}</p>}
    {review ? <div className="space-y-3">
      <h4 className="font-semibold">Review purchase correction</h4>
      <p>Purchase: {formatCents(purchase.amount_cents)} → {formatCents(review.amount_cents)}</p>
      {isShuttle && <p>Shuttles: {purchase.units} → {review.units}</p>}
      <p>Bank adjustment: {formatCents(purchase.amount_cents - review.amount_cents)}</p>
      <p>{isShuttle ? 'Shuttle value' : 'Court credit'} adjustment: {formatCents(review.amount_cents - purchase.amount_cents)}</p>
      <p className="break-words whitespace-pre-wrap">Reason: {review.reason}</p>
      <p className="text-sm text-slate-600">The original purchase will be reversed and replaced. Its purchase date is retained. Previously settled member charges stay as recorded.</p>
      <button type="button" className="underline" disabled={saving} onClick={() => { setReview(null); setError(null); }}>Back to edit</button>
      <FormActionRow onCancel={onCancel} disabled={saving}><button className="btn-primary" disabled={saving} onClick={() => void save()}>{saving ? 'Saving…' : 'Save purchase correction'}</button></FormActionRow>
    </div> : <form onSubmit={prepare} className="space-y-3">
      <div><label className="label" htmlFor={`purchase-amount-${purchase.id}`}>Correct purchase amount (AUD)</label>
        <input autoFocus className="input" id={`purchase-amount-${purchase.id}`} inputMode="decimal" value={amount} onChange={event => setAmount(event.target.value)} required /></div>
      {isShuttle && <div><label className="label" htmlFor={`purchase-units-${purchase.id}`}>Correct shuttle count</label>
        <input className="input" id={`purchase-units-${purchase.id}`} type="number" min="1" step="1" value={units} onChange={event => setUnits(event.target.value)} required /></div>}
      <div><label className="label" htmlFor={`purchase-reason-${purchase.id}`}>Reason</label>
        <textarea className="input" id={`purchase-reason-${purchase.id}`} maxLength={1000} value={reason} onChange={event => setReason(event.target.value)} required /></div>
      <FormActionRow onCancel={onCancel}><button type="submit" className="btn-primary">Review correction</button></FormActionRow>
    </form>}
  </div>;
}
