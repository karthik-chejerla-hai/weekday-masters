import { useState } from 'react';
import { api } from '../../services/api';
import { parseDollarsToCents } from './format';

interface AssetPurchaseFormsProps {
  onRecorded: () => void;
}

/**
 * Recording what the club bought.
 *
 * Neither of these moves a player balance — they convert cash into something the
 * club will consume later. Keeping them next to the position is deliberate: the
 * moment you notice credit is short is the moment you want to top it up.
 */
export default function AssetPurchaseForms({ onRecorded }: AssetPurchaseFormsProps) {
  return (
    <div className="space-y-4">
      <CourtCreditForm onRecorded={onRecorded} />
      <ShuttlePurchaseForm onRecorded={onRecorded} />
    </div>
  );
}

function CourtCreditForm({ onRecorded }: { onRecorded: () => void }) {
  // The venue sells credit in fixed blocks, so this is almost always $100.
  const [amount, setAmount] = useState('100.00');
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const cents = parseDollarsToCents(amount);
    if (!Number.isFinite(cents) || cents <= 0) return;

    setIsSaving(true);
    setError(null);
    try {
      await api.recordCourtCredit(cents, 'Venue account top-up');
      onRecorded();
    } catch {
      setError('Could not record that top-up.');
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="card p-4 space-y-3">
      <h3 className="font-semibold text-slate-900">Top up court credit</h3>
      <div>
        <label className="label" htmlFor="court-credit-amount">Amount paid to the venue</label>
        <input
          id="court-credit-amount"
          className="input"
          type="number"
          step="0.01"
          min="0.01"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
        />
      </div>
      {error && <p className="text-sm text-red-600">{error}</p>}
      <button type="submit" className="btn-primary w-full" disabled={isSaving}>
        {isSaving ? 'Recording…' : 'Record top-up'}
      </button>
    </form>
  );
}

function ShuttlePurchaseForm({ onRecorded }: { onRecorded: () => void }) {
  const [units, setUnits] = useState('12');
  const [amount, setAmount] = useState('50.00');
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const cents = parseDollarsToCents(amount);
    const count = Number.parseInt(units, 10);
    if (!Number.isFinite(cents) || cents <= 0 || !Number.isInteger(count) || count <= 0) return;

    setIsSaving(true);
    setError(null);
    try {
      await api.recordShuttlePurchase(count, cents, 'Shuttle purchase');
      onRecorded();
    } catch {
      setError('Could not record that purchase.');
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="card p-4 space-y-3">
      <h3 className="font-semibold text-slate-900">Record shuttles bought</h3>
      <p className="text-xs text-slate-500">
        Buying at a different price blends the average. What is already in the bag keeps the
        value it was bought at.
      </p>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="label" htmlFor="shuttle-units">Shuttles</label>
          <input
            id="shuttle-units"
            className="input"
            type="number"
            min="1"
            value={units}
            onChange={(e) => setUnits(e.target.value)}
          />
        </div>
        <div>
          <label className="label" htmlFor="shuttle-cost">What they cost</label>
          <input
            id="shuttle-cost"
            className="input"
            type="number"
            step="0.01"
            min="0.01"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
          />
        </div>
      </div>
      {error && <p className="text-sm text-red-600">{error}</p>}
      <button type="submit" className="btn-primary w-full" disabled={isSaving}>
        {isSaving ? 'Recording…' : 'Record purchase'}
      </button>
    </form>
  );
}
