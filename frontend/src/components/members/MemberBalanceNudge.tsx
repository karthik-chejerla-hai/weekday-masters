import { useRef, useState } from 'react';
import { Bell, Loader2 } from 'lucide-react';
import { api } from '../../services/api';
import { balanceNudgeMessage } from '../../services/balance-nudge';
import { formatCents } from '../money/format';
import type { PlayerBalance } from '../../types';

export default function MemberBalanceNudge({ balance }: { balance: PlayerBalance }) {
  const requestId = useRef<string>();
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);
  const [message, setMessage] = useState('');
  const [error, setError] = useState(false);
  if (balance.balance_cents >= 2000) return null;
  async function nudge() {
    setBusy(true); setError(false); setMessage('');
    requestId.current ||= crypto.randomUUID();
    try {
      const result = await api.nudgeBalance(balance.user_id, requestId.current);
      setMessage(balanceNudgeMessage(result, balance.name)); setDone(true);
    } catch (failure) {
      setError(true);
      setMessage((failure as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Could not record this reminder. Try again.');
    } finally { setBusy(false); }
  }
  return <div className="mt-3 border-t border-slate-100 pt-3 text-sm">
    <div className="flex flex-wrap items-center gap-3">
      <span>Balance: {formatCents(balance.balance_cents)}</span>
      <button type="button" disabled={busy || done} onClick={() => void nudge()} className="inline-flex min-h-11 items-center gap-1 text-primary-700 disabled:text-slate-400">
        {busy ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" /> : <Bell className="h-4 w-4" aria-hidden="true" />}
        {busy ? 'Recording reminder…' : done ? 'Reminder recorded' : 'Nudge to top up'}
      </button>
    </div>
    {message && <p role={error ? 'alert' : 'status'} className={error ? 'text-red-600' : 'text-slate-600'}>{message}</p>}
  </div>;
}
