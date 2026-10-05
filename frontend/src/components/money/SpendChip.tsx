import { Link } from 'react-router-dom';
import { usePersonalSpend } from '../../hooks/usePersonalSpend';
import { formatCents } from './format';

export default function SpendChip() {
  const { data, error, isLoading } = usePersonalSpend();
  if (!data && !error && !isLoading) return null;

  return (
    <Link
      to="/money?tab=analytics"
      aria-label={data ? `Your year-to-date spend: ${formatCents(data.ytd_cents)}. Open Analytics` : 'Your spend. Open Analytics'}
      className="inline-flex min-h-10 flex-col items-center justify-center rounded-xl border border-slate-200 bg-slate-50 px-2.5 py-1 text-slate-700 focus:outline-none focus:ring-2 focus:ring-primary-600 focus:ring-offset-2"
    >
      <span className="text-[10px] font-medium leading-tight">{data ? `${data.year} spend` : 'Your spend'}</span>
      <span className="whitespace-nowrap text-xs font-semibold tabular-nums">
        {data ? formatCents(data.ytd_cents) : error ? 'View details' : 'Loading…'}
      </span>
    </Link>
  );
}
