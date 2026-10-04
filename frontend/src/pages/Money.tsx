import { useCallback, useEffect, useState } from 'react';
import { Loader2 } from 'lucide-react';
import { useAuth } from '../context/useAuth';
import { api } from '../services/api';
import BalancesList from '../components/money/BalancesList';
import LedgerBrowser from '../components/money/LedgerBrowser';
import TopupForm from '../components/money/TopupForm';
import BalanceChip from '../components/money/BalanceChip';
import PositionPanel from '../components/money/PositionPanel';
import AssetPurchaseForms from '../components/money/AssetPurchaseForms';
import type { ClubPosition, MyBalance, PlayerBalance } from '../types';

type Tab = 'balances' | 'ledger' | 'club';

const TABS: Array<{ id: Tab; label: string }> = [
  { id: 'balances', label: 'Balances' },
  { id: 'ledger', label: 'Ledger' },
  { id: 'club', label: 'Club assets' },
];

export default function Money() {
  const { user, isAdmin } = useAuth();
  const [tab, setTab] = useState<Tab>('balances');

  const [balances, setBalances] = useState<PlayerBalance[]>([]);
  const [myBalance, setMyBalance] = useState<MyBalance | null>(null);
  const [revision, setRevision] = useState(0);
  const [positionError, setPositionError] = useState(false);
  const [lowThreshold, setLowThreshold] = useState(2000);
  const [position, setPosition] = useState<ClubPosition | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setError(null);
    setPositionError(false);
    const [balanceList, mine, club, clubPosition] = await Promise.allSettled([
      api.listBalances(), api.getMyBalance(), api.getClub(), api.getClubPosition(),
    ]);
    if (balanceList.status === 'fulfilled') setBalances(balanceList.value);
    if (mine.status === 'fulfilled') setMyBalance(mine.value);
    if (balanceList.status === 'rejected' || mine.status === 'rejected') {
      setError('Could not load balances. Please try again.');
    }
    if (club.status === 'fulfilled' && typeof club.value?.low_balance_threshold_cents === 'number') {
      setLowThreshold(club.value.low_balance_threshold_cents);
    }
    if (clubPosition.status === 'fulfilled' && clubPosition.value) {
      setPosition(clubPosition.value);
    } else {
      setPositionError(true);
    }
    setRevision((current) => current + 1);
    setIsLoading(false);
  }, []);

  useEffect(() => { void load(); }, [load]);

  const activeTab = tab;

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

      <div className="grid grid-cols-3 gap-1 rounded-xl bg-slate-100 p-1" role="tablist" aria-label="Money views">
        {TABS.map(({ id, label }) => (
          <button
            key={id}
            role="tab"
            aria-selected={activeTab === id}
            onClick={() => setTab(id)}
            className={`min-h-11 rounded-lg px-2 py-2 text-sm font-semibold transition-colors ${
              activeTab === id
                ? 'bg-white text-slate-950 shadow-sm'
                : 'text-slate-500 hover:text-slate-800'
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {activeTab === 'balances' && (
        <div className="space-y-6">
          <BalancesList
            balances={balances}
            lowThresholdCents={lowThreshold}
            currentUserId={user?.id}
          />
          {isAdmin && <TopupForm members={balances} onRecorded={load} />}
        </div>
      )}

      {activeTab === 'ledger' && <LedgerBrowser userId={user?.id} revision={revision} />}

      {activeTab === 'club' && (
        <div className="space-y-6">
          {positionError && <div role="alert" className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700">
            Could not load club assets.
            <button className="ml-2 font-semibold underline" onClick={() => void load()}>Try again</button>
          </div>}
          {position && <PositionPanel position={position} />}
          {isAdmin && position && !position.assets_pending && <AssetPurchaseForms onRecorded={load} />}
        </div>
      )}
    </div>
  );
}
