import { useEffect, useState } from 'react';
import { isAxiosError } from 'axios';
import { api } from '../../services/api';
import type { BalanceCorrection, ClubPosition } from '../../types';
import { formatCents } from './format';

type Asset = 'court_credit' | 'bank';
type Review = Parameters<typeof api.correctAssetBalance>[0];
const label = (asset: Asset) => asset === 'court_credit' ? 'Court credit' : 'Bank';

export default function BalanceCorrectionForm({ position, revision, onRecorded }: {
  position: ClubPosition; revision: number; onRecorded: () => void;
}) {
  const [asset, setAsset] = useState<Asset>('court_credit');
  const [amount, setAmount] = useState('');
  const [reason, setReason] = useState('');
  const [review, setReview] = useState<Review | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [history, setHistory] = useState<BalanceCorrection[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [historyError, setHistoryError] = useState(false);
  const [page, setPage] = useState(0);
  const [retry, setRetry] = useState(0);
  const current = asset === 'court_credit' ? position.assets.court_credit_cents : position.assets.bank_cents;

  useEffect(() => {
    let active = true;
    setLoading(true);
    setHistoryError(false);
    api.listBalanceCorrections(20, page * 20).then((result) => {
      if (active) { setHistory(result.items); setTotal(result.total); }
    }).catch(() => { if (active) setHistoryError(true); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [revision, page, retry]);

  const prepare = (event: React.FormEvent) => {
    event.preventDefault();
    setError(null); setSuccess(null);
    // Parse decimal text exactly; never round a third decimal place.
    const match = /^(\d+)(?:\.(\d{1,2}))?$/.exec(amount.trim());
    const cents = match ? Number(match[1]) * 100 + Number((match[2] ?? '').padEnd(2, '0')) : NaN;
    if (!Number.isSafeInteger(cents) || cents < 0 || cents > 4503599627370495) { setError('Enter a non-negative amount with up to two decimal places.'); return; }
    if (cents === current) { setError('The new balance is already the recorded balance.'); return; }
    if (!reason.trim()) { setError('Enter a reason for the correction.'); return; }
    setReview({request_id: crypto.randomUUID(), account_kind:asset, expected_cents:current, target_cents:cents, reason:reason.trim()});
  };

  const save = async () => {
    if (!review || saving) return;
    setSaving(true); setError(null);
    try {
      await api.correctAssetBalance(review);
      setSuccess(`${label(review.account_kind)} corrected to ${formatCents(review.target_cents)}. Audit record saved.`);
      setReview(null); setAmount(''); setReason(''); setPage(0);
      onRecorded();
    } catch (err) {
      setError(isAxiosError<{ message?: string }>(err) ? err.response?.data?.message ?? 'Could not save. Retry this correction safely.' : 'Could not save. Retry this correction safely.');
      if (isAxiosError<{code?: string}>(err) && err.response?.data?.code === 'balance_changed') { setReview(null); onRecorded(); }
    } finally { setSaving(false); }
  };

  return <section className="card p-4 space-y-4" aria-label="Balance corrections">
    <div>
      <h3 className="font-semibold text-slate-900">Correct an asset balance</h3>
      <p className="mt-1 text-sm text-slate-600">Use the verified balance from your bank or venue. The adjustment also changes club surplus. Member balances and the other assets stay as recorded.</p>
    </div>
    {success && <p role="status" className="text-sm text-green-700">{success}</p>}
    {error && <p role="alert" className="text-sm text-red-600">{error}</p>}
    {review ? <div className="space-y-3 rounded-lg bg-slate-50 p-4">
      <h4 className="font-semibold">Review {label(review.account_kind).toLowerCase()} correction</h4>
      <dl className="grid grid-cols-2 gap-2 text-sm">
        <dt>Recorded balance</dt><dd>{formatCents(review.expected_cents)}</dd>
        <dt>Correct balance</dt><dd>{formatCents(review.target_cents)}</dd>
        <dt>Asset and surplus adjustment</dt><dd>{formatCents(review.target_cents - review.expected_cents)}</dd>
      </dl>
      <p className="whitespace-pre-wrap break-words text-sm">Reason: {review.reason}</p>
      <p className="text-xs text-slate-500">Your name, time, reason and before/after balances will be retained in the audit history. Existing ledger entries are preserved.</p>
      <div className="flex flex-wrap gap-2">
        <button type="button" className="btn-primary" disabled={saving} onClick={() => void save()}>{saving ? 'Saving…' : 'Save correction'}</button>
        <button type="button" className="btn-secondary" disabled={saving} onClick={() => { setReview(null); setError(null); }}>Back to edit</button>
      </div>
    </div> : <form onSubmit={prepare} className="space-y-3">
      <div className="grid gap-3 sm:grid-cols-2">
        <div><label className="label" htmlFor="correction-asset">Asset</label>
          <select id="correction-asset" className="input" value={asset} onChange={(event) => setAsset(event.target.value as Asset)}><option value="court_credit">Court credit</option><option value="bank">Bank</option></select>
        </div>
        <div><label className="label" htmlFor="correction-amount">Correct balance (AUD)</label>
          <input id="correction-amount" className="input" inputMode="decimal" value={amount} onChange={(event) => setAmount(event.target.value)} required />
        </div>
      </div>
      <p className="text-sm text-slate-500">Recorded {label(asset).toLowerCase()}: {formatCents(current)}</p>
      <div><label className="label" htmlFor="correction-reason">Reason</label>
        <textarea id="correction-reason" className="input" maxLength={1000} rows={3} value={reason} onChange={(event) => setReason(event.target.value)} required />
      </div>
      <button className="btn-secondary" type="submit">Review correction</button>
    </form>}
    <div className="border-t border-slate-200 pt-4 space-y-3">
      <h4 className="font-semibold">Correction audit history</h4>
      {loading && <p role="status" className="text-sm text-slate-500">Loading corrections…</p>}
      {historyError && <p role="alert" className="text-sm text-red-600">Could not load correction history. <button className="underline" onClick={() => setRetry((value) => value + 1)}>Try again</button></p>}
      {!loading && !historyError && history.length === 0 && <p className="text-sm text-slate-500">No corrections on this page.</p>}
      {history.map((item) => <article key={item.request_id} className="rounded-lg border border-slate-200 p-3 text-sm space-y-1">
        <p className="font-medium">{label(item.account_kind)}: {formatCents(item.before_cents)} → {formatCents(item.after_cents)}</p>
        <p>Asset and surplus adjustment: {formatCents(item.adjustment_cents)}</p>
        <p className="whitespace-pre-wrap break-words">{item.reason}</p>
        <p className="text-xs text-slate-500">{item.actor_name} · {new Date(item.created_at).toLocaleString('en-AU', { timeZone:'Australia/Sydney' })} (Sydney)</p>
        <p className="break-all text-xs text-slate-500">Transaction: {item.transaction_id}</p>
      </article>)}
      <div className="flex gap-2">
        {page > 0 && <button className="btn-secondary" disabled={loading} onClick={() => setPage((value) => value - 1)}>Newer corrections</button>}
        {(page + 1) * 20 < total && <button className="btn-secondary" disabled={loading} onClick={() => setPage((value) => value + 1)}>Older corrections</button>}
      </div>
    </div>
  </section>;
}
