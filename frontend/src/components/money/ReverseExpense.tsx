import { useEffect, useId, useRef, useState } from 'react';
import { Loader2, RotateCcw } from 'lucide-react';
import { api } from '../../services/api';
import type { LedgerEntryView } from '../../types';
import { assistantError } from '../assistant/errors';

/**
 * Admin action on a shared expense row. The ledger is append-only, so this
 * posts a reversing transaction for the whole expense, not for this member's
 * row alone, and it asks first because every member involved is notified.
 */
export default function ReverseExpense({ entry, onReversed }: { entry: LedgerEntryView; onReversed: (transactionId: string, name: string) => void }) {
  const id = useId();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const locked = useRef(false);
  const opener = useRef<HTMLButtonElement>(null);
  const question = useRef<HTMLDivElement>(null);
  const wasOpen = useRef(false);
  const name = entry.description.trim() || 'this expense';

  // The opener and the question replace each other, so focus would fall back to
  // the page. Move it to the question when it opens, and back when it closes.
  useEffect(() => {
    if (open) question.current?.focus();
    else if (wasOpen.current) opener.current?.focus();
    wasOpen.current = open;
  }, [open]);

  const reverse = async () => {
    if (locked.current) return;
    locked.current = true; setBusy(true); setError('');
    try {
      await api.reverseTransaction(entry.transaction_id, `Reversal of ${name}`);
      // The parent marks the row reversed and announces it; this component then unmounts.
      onReversed(entry.transaction_id, name);
    } catch (err) {
      setError(assistantError(err, 'Could not reverse the expense. Try again.'));
    } finally { locked.current = false; setBusy(false); }
  };

  if (!open) return <button ref={opener} type="button" className="mt-2 inline-flex min-h-11 items-center gap-1.5 text-xs font-medium text-slate-600 underline" aria-label={`Reverse ${name}`} onClick={() => setOpen(true)}>
    <RotateCcw className="h-3.5 w-3.5" aria-hidden="true" />Reverse
  </button>;

  return <div ref={question} tabIndex={-1} role="group" aria-labelledby={id} className="mt-3 space-y-3 rounded-xl border border-amber-200 bg-amber-50 p-3 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-400">
    <p id={id} className="font-semibold text-amber-950">Reverse {name}?</p>
    <p className="text-amber-900">This undoes the whole expense for every member who shared it, and for the payer. Each of them is notified. To correct a mistake, reverse it and then record the expense again.</p>
    {error && <p role="alert" className="text-red-800">{error}</p>}
    <div className="flex flex-wrap gap-2">
      <button type="button" className="btn-danger min-h-11 gap-2" disabled={busy} onClick={() => void reverse()}>{busy && <Loader2 className="h-4 w-4 animate-spin" />}{busy ? 'Reversing…' : 'Reverse expense'}</button>
      <button type="button" className="btn-outline min-h-11" disabled={busy} onClick={() => { setOpen(false); setError(''); }}>Cancel</button>
    </div>
  </div>;
}
