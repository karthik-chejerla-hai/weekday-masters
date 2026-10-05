import { Link } from 'react-router-dom';
import { usePersonalSpend } from '../../hooks/usePersonalSpend';
import { formatCents } from './format';
import ValueBadge from '../ui/ValueBadge';

export default function SpendChip() {
  const { data, error, isLoading } = usePersonalSpend();
  if (!data && !error && !isLoading) return null;

  return (
    <Link
      to="/money?tab=analytics"
      aria-label={data ? `Your year-to-date spend: ${formatCents(data.ytd_cents)}. Open Analytics` : 'Your spend. Open Analytics'}
      className="inline-flex min-h-11 items-center rounded-md focus:outline-none focus:ring-2 focus:ring-primary-600 focus:ring-offset-2"
    >
      <ValueBadge
        label="YTD spend"
        value={data ? formatCents(data.ytd_cents) : error ? 'View details' : 'Loading…'}
        tone={data ? 'primary' : 'neutral'}
        title={data ? `Your ${data.year} year-to-date session charges` : 'Your year-to-date session charges'}
      />
    </Link>
  );
}
