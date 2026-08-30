import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { ArrowLeft, Loader2 } from 'lucide-react';
import { format, parseISO } from 'date-fns';
import { api } from '../services/api';
import SettlementBreakdown from '../components/settlement/SettlementBreakdown';
import type { SettlementView } from '../types';

/**
 * What a past session cost, readable by any member.
 *
 * Deliberately not admin-only: the people in a split are the people best placed
 * to notice if it is wrong, and the club already worked this way in Splitwise.
 */
export default function SessionSettlement() {
  const { id } = useParams<{ id: string }>();
  const [view, setView] = useState<SettlementView | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    api
      .getSessionSettlement(id)
      .then(setView)
      .catch(() => setError('This session has not been settled yet.'))
      .finally(() => setIsLoading(false));
  }, [id]);

  if (isLoading) {
    return (
      <div className="card p-8 flex items-center justify-center">
        <Loader2 className="w-8 h-8 text-primary-600 animate-spin" />
      </div>
    );
  }

  if (error || !view) {
    return (
      <div className="space-y-4">
        <BackLink />
        <div className="card p-8 text-center text-slate-600">{error}</div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <BackLink />

      <div>
        <h1 className="text-lg font-semibold text-slate-900">{view.session.title}</h1>
        {view.session.ends_at && (
          <p className="text-sm text-slate-500 mt-0.5">
            {format(parseISO(view.session.ends_at), 'EEEE d MMMM yyyy')}
          </p>
        )}
      </div>

      {view.reversed_at && (
        <div className="rounded-lg border border-secondary-300 bg-secondary-50 p-3 text-sm text-secondary-900">
          This settlement was reversed on {format(parseISO(view.reversed_at), 'd MMM yyyy')}.
        </div>
      )}

      <SettlementBreakdown preview={view} rates={view.rates} showRates />
    </div>
  );
}

function BackLink() {
  return (
    <Link
      to="/sessions"
      className="inline-flex items-center gap-1 text-sm font-medium text-primary-600 hover:text-primary-700"
    >
      <ArrowLeft className="w-4 h-4" />
      Sessions
    </Link>
  );
}
