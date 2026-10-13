import { useEffect, useId, useRef, useState } from 'react';
import { api } from '../../services/api';
import { parseDollarsToCents } from './format';
import PositionPanel from './PositionPanel';
import BalanceCorrectionForm from './BalanceCorrectionForm';
import FormActionRow from './FormActionRow';
import type { ClubPosition } from '../../types';

const actions = ['Record court credit purchase', 'Record shuttle purchase', 'Audit bank balance'];

/** One form follows its card on mobile and spans below the card row on desktop. */
export default function AssetPurchaseForms({ position, revision, onRecorded }: {
  position: ClubPosition; revision: number; onRecorded: () => void;
}) {
  const [active, setActive] = useState<number | null>(null);
  const [saving, setSaving] = useState(false);
  const [success, setSuccess] = useState<{ asset: number; message: string } | null>(null);
  const panelId = useId();
  const panel = useRef<HTMLDivElement>(null);
  const openers = useRef<Array<HTMLButtonElement | null>>([]);
  const returnTo = useRef<number | null>(null);

  useEffect(() => {
    if (active !== null) panel.current?.querySelector<HTMLElement>('select, input, textarea')?.focus();
    else if (returnTo.current !== null) {
      openers.current[returnTo.current]?.focus();
      returnTo.current = null;
    }
  }, [active]);

  const close = () => { returnTo.current = active; setActive(null); };
  const recorded = () => {
    if (active !== null) setSuccess({ asset: active, message: active === 0 ? 'Court credit purchase recorded.' : active === 1 ? 'Shuttle purchase recorded.' : 'Balance correction recorded.' });
    setSaving(false);
    close();
    onRecorded();
  };

  return <PositionPanel position={position} formAssetIndex={active ?? success?.asset} actions={actions.map((label, index) => (
    <button key={label} ref={element => { openers.current[index] = element; }} type="button"
      className="btn-outline w-full" aria-expanded={active === index} aria-controls={panelId} disabled={saving}
      onClick={() => { setSuccess(null); if (active === index) close(); else setActive(index); }}>
      {label}
    </button>
  ))}>
    {(active !== null || success) && <>
      {success && <p role="status" className="text-sm text-green-800">{success.message}</p>}
      <div id={panelId} ref={panel} role="region" aria-label={active === null ? 'Asset form' : actions[active]} hidden={active === null} className="space-y-3">
        {active === 0 && <CourtCreditForm onRecorded={recorded} onSavingChange={setSaving} onCancel={close} />}
        {active === 1 && <ShuttlePurchaseForm onRecorded={recorded} onSavingChange={setSaving} onCancel={close} />}
        {active === 2 && <BalanceCorrectionForm position={position} revision={revision} initialAsset="bank"
          onRecorded={onRecorded} onSaved={recorded} onSavingChange={setSaving} onCancel={close} />}
      </div>
    </>}
  </PositionPanel>;
}

function CourtCreditForm({ onRecorded, onSavingChange, onCancel }: { onRecorded: () => void; onSavingChange: (saving: boolean) => void; onCancel: () => void }) {
  // The venue sells credit in fixed blocks, so this is almost always $100.
  const [amount, setAmount] = useState('100.00');
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const cents = parseDollarsToCents(amount);
    if (!Number.isFinite(cents) || cents <= 0) return;

    setIsSaving(true);
    onSavingChange(true);
    setError(null);
    try {
      await api.recordCourtCredit(cents, 'Venue account top-up');
      onRecorded();
    } catch {
      setError('Could not record that top-up.');
    } finally {
      setIsSaving(false);
      onSavingChange(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="card p-4 space-y-3">
      <h3 className="font-semibold text-slate-900">Record court credit purchase</h3>
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
      {error && <p role="alert" className="text-sm text-red-600">{error}</p>}
      <FormActionRow onCancel={onCancel} disabled={isSaving}>
        <button type="submit" className="btn-primary min-w-0 flex-1 sm:flex-none" disabled={isSaving}>
          {isSaving ? 'Recording…' : 'Record purchase'}
        </button>
      </FormActionRow>
    </form>
  );
}

function ShuttlePurchaseForm({ onRecorded, onSavingChange, onCancel }: { onRecorded: () => void; onSavingChange: (saving: boolean) => void; onCancel: () => void }) {
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
    onSavingChange(true);
    setError(null);
    try {
      await api.recordShuttlePurchase(count, cents, 'Shuttle purchase');
      onRecorded();
    } catch {
      setError('Could not record that purchase.');
    } finally {
      setIsSaving(false);
      onSavingChange(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="card p-4 space-y-3">
      <h3 className="font-semibold text-slate-900">Record shuttle purchase</h3>
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
      {error && <p role="alert" className="text-sm text-red-600">{error}</p>}
      <FormActionRow onCancel={onCancel} disabled={isSaving}>
        <button type="submit" className="btn-primary min-w-0 flex-1 sm:flex-none" disabled={isSaving}>
          {isSaving ? 'Recording…' : 'Record purchase'}
        </button>
      </FormActionRow>
    </form>
  );
}
