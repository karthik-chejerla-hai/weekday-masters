const VALUE_TONE = {
  primary: 'bg-primary-700 text-white',
  positive: 'bg-emerald-700 text-white',
  warning: 'bg-amber-700 text-white',
  negative: 'bg-red-700 text-white',
  neutral: 'bg-slate-500 text-white',
};

interface ValueBadgeProps {
  label: string;
  value: string;
  tone?: keyof typeof VALUE_TONE;
  title?: string;
}

export default function ValueBadge({ label, value, tone = 'primary', title }: ValueBadgeProps) {
  return (
    <span className="inline-flex shrink-0 overflow-hidden whitespace-nowrap rounded-md text-xs leading-5 shadow-sm" title={title}>
      <span className="bg-slate-700 px-2.5 py-1 font-medium text-white">{label}</span>
      <span className={`px-2.5 py-1 font-semibold tabular-nums ${VALUE_TONE[tone]}`}>{value}</span>
    </span>
  );
}
