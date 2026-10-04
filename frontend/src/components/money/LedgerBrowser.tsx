import { useEffect, useRef, useState } from 'react';
import { Loader2 } from 'lucide-react';
import { api } from '../../services/api';
import type { LedgerActivityView } from '../../types';
import LedgerList from './LedgerList';

const PAGE_SIZE = 50;

export default function LedgerBrowser({ userId, revision }: { userId?: string; revision: number }) {
  const [mineOnly, setMineOnly] = useState(true);
  const [topupsOnly, setTopupsOnly] = useState(false);
  const [entries, setEntries] = useState<LedgerActivityView[]>([]);
  const [total, setTotal] = useState(0);
  const [nextOffset, setNextOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [retry, setRetry] = useState(0);
  const generation = useRef(0);
  const pagePending = useRef(false);

  useEffect(() => {
    const request = ++generation.current;
    pagePending.current = false;
    setEntries([]);
    setTotal(0);
    setNextOffset(0);
    setLoading(true);
    setError(false);
    api.getLedgerActivity(mineOnly ? 'mine' : 'all', topupsOnly, PAGE_SIZE, 0)
      .then((page) => {
        if (request !== generation.current) return;
        setEntries(page.items);
        setTotal(page.total);
        setNextOffset(page.items.length);
      })
      .catch(() => { if (request === generation.current) setError(true); })
      .finally(() => { if (request === generation.current) setLoading(false); });
    return () => { generation.current = request + 1; };
  }, [mineOnly, topupsOnly, userId, revision, retry]);

  const loadMore = async () => {
    if (loading || pagePending.current) return;
    const request = generation.current;
    pagePending.current = true;
    setLoading(true);
    setError(false);
    try {
      const page = await api.getLedgerActivity(mineOnly ? 'mine' : 'all', topupsOnly, PAGE_SIZE, nextOffset);
      if (request !== generation.current) return;
      setEntries((current) => {
        const seen = new Set(current.map((entry) => entry.id));
        return [...current, ...page.items.filter((entry) => !seen.has(entry.id))];
      });
      setTotal(page.total);
      // Offset counts fetched rows, including duplicates after a concurrent
      // append. Counting only visible rows can get stuck on the last page.
      setNextOffset(page.items.length ? nextOffset + page.items.length : Math.max(nextOffset, page.total));
    } catch {
      if (request === generation.current) setError(true);
    } finally {
      if (request === generation.current) {
        pagePending.current = false;
        setLoading(false);
      }
    }
  };

  return (
    <section className="space-y-4" aria-label="Ledger history">
      <div className="flex flex-wrap gap-x-6 gap-y-3 rounded-xl border border-slate-200 bg-white p-4">
        <Toggle checked={mineOnly} onChange={setMineOnly} label="Show my transactions only" />
        <Toggle checked={topupsOnly} onChange={setTopupsOnly} label="Top-ups only" />
      </div>
      <p className="text-sm text-slate-500">
        {mineOnly ? 'Your transactions' : 'All member transactions, including inactive members'}.
        {' '}Expand a game to see each member’s share and balance.
      </p>
      {entries.length > 0 && <LedgerList entries={entries} showMember={!mineOnly} userId={userId} />}
      {error && (
        <div role="alert" className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700">
          Could not load transactions. Your history is still saved.
          <button className="ml-2 font-semibold underline" onClick={() => entries.length ? void loadMore() : setRetry((value) => value + 1)}>
            Try again
          </button>
        </div>
      )}
      {!loading && !error && entries.length === 0 && (
        <p className="py-8 text-center text-sm text-slate-500">No transactions match these filters.</p>
      )}
      <div className="flex flex-col items-center gap-3">
        {loading && <span role="status" className="flex items-center gap-2 text-sm text-slate-500"><Loader2 className="h-4 w-4 animate-spin" />Loading transactions…</span>}
        {!error && total > 0 && (
          <p className="text-xs text-slate-500" aria-live="polite">
            {nextOffset >= total && entries.length < total
              ? `${entries.length} entries loaded`
              : `${entries.length} of ${total} entries`}{nextOffset >= total ? ' · End of history' : ''}
          </p>
        )}
        {!error && entries.length > 0 && nextOffset < total && (
          <button className="btn-secondary min-h-11" disabled={loading} onClick={() => void loadMore()}>Load older transactions</button>
        )}
      </div>
    </section>
  );
}

function Toggle({ checked, onChange, label }: { checked: boolean; onChange: (value: boolean) => void; label: string }) {
  return (
    <label className="flex min-h-11 cursor-pointer items-center gap-3 text-sm font-medium text-slate-700">
      <input type="checkbox" role="switch" className="peer sr-only" checked={checked} onChange={(event) => onChange(event.target.checked)} />
      <span className={`relative h-6 w-10 shrink-0 rounded-full transition-colors peer-focus-visible:ring-2 peer-focus-visible:ring-primary-500 peer-focus-visible:ring-offset-2 ${checked ? 'bg-primary-600' : 'bg-slate-300'}`} aria-hidden="true">
        <span className={`absolute top-1 h-4 w-4 rounded-full bg-white shadow-sm transition-transform ${checked ? 'translate-x-5' : 'translate-x-1'}`} />
      </span>
      {label}
    </label>
  );
}
