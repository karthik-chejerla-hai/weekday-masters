import { useEffect, useId, useRef, useState } from 'react';
import type { ReactNode } from 'react';

interface FormDisclosureProps {
  label: string;
  successMessage: string;
  onRecorded: () => void;
  children: (actions: { onRecorded: () => void; onSavingChange: (saving: boolean) => void }) => ReactNode;
}

/** Keep financial forms idle until requested, with a safe return to their opener. */
export default function FormDisclosure({ label, successMessage, onRecorded, children }: FormDisclosureProps) {
  const id = useId();
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [success, setSuccess] = useState(false);
  const opener = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const returnFocus = useRef(false);

  useEffect(() => {
    if (open) {
      panel.current?.querySelector<HTMLElement>('select, input, textarea')?.focus();
    } else if (returnFocus.current) {
      opener.current?.focus();
      returnFocus.current = false;
    }
  }, [open]);

  const close = () => {
    returnFocus.current = true;
    setOpen(false);
  };
  const recorded = () => {
    setSaving(false);
    setSuccess(true);
    close();
    onRecorded();
  };

  return <div className="space-y-3">
    <button
      ref={opener} type="button" className="btn-outline" aria-expanded={open}
      aria-controls={id} disabled={saving}
      onClick={() => { if (open) close(); else { setSuccess(false); setOpen(true); } }}
    >{label}</button>
    {success && <p role="status" className="text-sm text-green-800">{successMessage}</p>}
    <div id={id} ref={panel} role="region" aria-label={label} hidden={!open} className="space-y-3">
      {open && <>
        {children({ onRecorded: recorded, onSavingChange: setSaving })}
        <button type="button" className="btn-secondary" disabled={saving} onClick={close}>Cancel</button>
      </>}
    </div>
  </div>;
}
