import { useCallback, useEffect, useState } from 'react';
import { Loader2, Wallet } from 'lucide-react';
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
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold text-slate-900 flex items-center gap-2">
          <Wallet className="w-5 h-5 text-primary-600" />
          Money
        </h1>
        {myBalance && <BalanceChip cents={myBalance.balance_cents} state={myBalance.state} />}
      </div>

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
          {error}
        </div>
      )}

      <div className="flex gap-1 border-b border-slate-200" role="tablist">
        {TABS.filter((t) => !t.adminOnly || isAdmin).map(({ id, label }) => (
          <button
            key={id}
            role="tab"
            aria-selected={tab === id}
            onClick={() => setTab(id)}
            className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors ${
              tab === id
                ? 'border-primary-600 text-primary-700'
                : 'border-transparent text-slate-500 hover:text-slate-700'
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
