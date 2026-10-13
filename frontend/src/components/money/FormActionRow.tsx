import type { ReactNode } from 'react';

/** Keep the form's primary action and neutral cancellation together. */
export default function FormActionRow({ children, onCancel, disabled = false }: {
  children: ReactNode; onCancel?: () => void; disabled?: boolean;
}) {
  return <div className="flex items-stretch gap-3">
    {children}
    {onCancel && <button type="button" className="btn-ghost shrink-0 border border-slate-300 bg-white" disabled={disabled} onClick={onCancel}>Cancel</button>}
  </div>;
}
