import { useCallback, useEffect, useState } from 'react';
import { Loader2 } from 'lucide-react';
import { useAuth } from '../context/useAuth';
import { api } from '../services/api';
import BalancesList from '../components/money/BalancesList';
import LedgerList from '../components/money/LedgerList';
import TopupForm from '../components/money/TopupForm';
import BalanceChip from '../components/money/BalanceChip';
import PositionPanel from '../components/money/PositionPanel';
import AssetPurchaseForms from '../components/money/AssetPurchaseForms';
import type { ClubPosition, LedgerEntryView, MyBalance, PlayerBalance } from '../types';

type Tab = 'balances' | 'ledger' | 'club';

const TABS: Array<{ id: Tab; label: string; adminOnly?: boolean }> = [
  { id: 'balances', label: 'Balances' },
  { id: 'ledger', label: 'My ledger' },
  { id: 'club', label: 'Club assets', adminOnly: true },
];

export default function Money() {
  const { user, isAdmin } = useAuth();
  const [tab, setTab] = useState<Tab>('balances');

  const [balances, setBalances] = useState<PlayerBalance[]>([]);
  const [myBalance, setMyBalance] = useState<MyBalance | null>(null);
  const [entries, setEntries] = useState<LedgerEntryView[]>([]);
  const [lowThreshold, setLowThreshold] = useState(2000);
  const [position, setPosition] = useState<ClubPosition | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setError(null);
    try {
      const [balanceList, mine, history] = await Promise.all([
        api.listBalances(),
        api.getMyBalance(),
        api.getMyEntries(),
      ]);
      setBalances(balanceList);
      setMyBalance(mine);
      setEntries(history.items);

      // The threshold is a club setting, so other members' chips use the same
      // rule the server applied to ours.
      if (isAdmin) {
        try {
          const [club, clubPosition] = await Promise.all([api.getClub(), api.getClubPosition()]);
          if (typeof club.low_balance_threshold_cents === 'number') {
            setLowThreshold(club.low_balance_threshold_cents);
          }
          setPosition(clubPosition);
        } catch {
          // Non-fatal: the balances still render without the club's own figures.
        }
      }
    } catch {
      setError('Could not load balances. Pull to refresh, or try again shortly.');
    } finally {
      setIsLoading(false);
    }
  }, [isAdmin]);

  useEffect(() => {
    load();
  }, [load]);

  if (isLoading) {
    return (
      <div className="card p-8 flex items-center justify-center">
        <Loader2 className="w-8 h-8 text-primary-600 animate-spin" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div className="page-heading mb-0">
          <p className="page-kicker">Club finances</p>
          <h1 className="page-title">Money</h1>
          <p className="page-description">Check member balances and understand every change to yours.</p>
        </div>
        {myBalance && (
          <div className="shrink-0 text-right">
            <p className="mb-1 text-xs font-medium text-slate-500">Your balance</p>
            <BalanceChip cents={myBalance.balance_cents} state={myBalance.state} />
          </div>
        )}
      </div>

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
          {error}
        </div>
      )}

      <div className={`grid gap-1 rounded-xl bg-slate-100 p-1 ${isAdmin ? 'grid-cols-3' : 'grid-cols-2'}`} role="tablist" aria-label="Money views">
        {TABS.filter((t) => !t.adminOnly || isAdmin).map(({ id, label }) => (
          <button
            key={id}
            role="tab"
            aria-selected={tab === id}
            onClick={() => setTab(id)}
            className={`min-h-11 rounded-lg px-2 py-2 text-sm font-semibold transition-colors ${
              tab === id
                ? 'bg-white text-slate-950 shadow-sm'
                : 'text-slate-500 hover:text-slate-800'
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {tab === 'balances' && (
        <div className="space-y-6">
          <BalancesList
            balances={balances}
            lowThresholdCents={lowThreshold}
            currentUserId={user?.id}
          />
          {isAdmin && <TopupForm members={balances} onRecorded={load} />}
        </div>
      )}

      {tab === 'ledger' && <LedgerList entries={entries} />}

      {tab === 'club' && isAdmin && (
        <div className="space-y-6">
          {position && <PositionPanel position={position} />}
          <AssetPurchaseForms onRecorded={load} />
        </div>
      )}
    </div>
  );
}
