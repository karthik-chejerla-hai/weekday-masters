import { useState } from 'react';
import { api } from '../../services/api';
import { parseDollarsToCents } from './format';
import type { PlayerBalance } from '../../types';

interface TopupFormProps {
  members: PlayerBalance[];
  onRecorded: () => void;
}

/**
 * Record money a member has transferred to the club.
 *
 * The club is paid by bank transfer out of band; this is the admin writing down
 * that it arrived. Amounts are entered in dollars and converted to cents once,
 * here, before anything else touches them.
 */
export default function TopupForm({ members, onRecorded }: TopupFormProps) {
  const [userId, setUserId] = useState('');
  const [amount, setAmount] = useState('');
  const [description, setDescription] = useState('');
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const amountCents = parseDollarsToCents(amount);
  const canSubmit = userId !== '' && Number.isFinite(amountCents) && amountCents > 0 && !isSaving;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;

    setIsSaving(true);
    setError(null);
    try {
      await api.recordTopup(userId, amountCents, description || 'Bank transfer');
      setUserId('');
      setAmount('');
      setDescription('');
      onRecorded();
    } catch {
      setError('Could not record that top-up. Check the amount and try again.');
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="card p-4 space-y-3">
      <h3 className="font-semibold text-slate-900">Record a top-up</h3>

      <div>
        <label className="label" htmlFor="topup-member">Member</label>
        <select
          id="topup-member"
          className="input"
          value={userId}
          onChange={(e) => setUserId(e.target.value)}
        >
          <option value="">Choose a member…</option>
          {members.map((m) => (
            <option key={m.user_id} value={m.user_id}>{m.name}</option>
          ))}
        </select>
      </div>

      <div>
        <label className="label" htmlFor="topup-amount">Amount</label>
        <input
          id="topup-amount"
          className="input"
          type="number"
          inputMode="decimal"
          step="0.01"
          min="0.01"
          placeholder="50.00"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
        />
      </div>

      <div>
        <label className="label" htmlFor="topup-note">Note</label>
        <input
          id="topup-note"
          className="input"
          type="text"
          placeholder="Bank transfer"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
      </div>

      {error && <p className="text-sm text-red-600">{error}</p>}

      <button type="submit" className="btn-primary w-full" disabled={!canSubmit}>
        {isSaving ? 'Recording…' : 'Record top-up'}
      </button>
    </form>
  );
}
