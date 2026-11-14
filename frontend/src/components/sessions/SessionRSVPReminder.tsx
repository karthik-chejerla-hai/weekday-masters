import { useEffect, useRef, useState } from 'react';
import { Bell } from 'lucide-react';
import { formatInTimeZone } from 'date-fns-tz';
import { api } from '../../services/api';
import { parseSydneyDateTime } from '../../utils/sydney-time';
import type { RSVPReminderPreview, Session } from '../../types';

export default function SessionRSVPReminder({ session, showAction = true }: { session: Session; showAction?: boolean }) {
  const [preview, setPreview] = useState<RSVPReminderPreview>();
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [error, setError] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [nextAllowedAt, setNextAllowedAt] = useState<string | null>(null);
  const [now, setNow] = useState(Date.now());
  const requestId = useRef<string>();
  const inFlight = useRef(false);
  const confirmButton = useRef<HTMLButtonElement>(null);
  const feedback = useRef<HTMLParagraphElement>(null);
  const wasConfirming = useRef(false);
  const triggerButton = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);
  useEffect(() => {
    if (preview) confirmButton.current?.focus();
    else if (wasConfirming.current && !triggerButton.current?.disabled) triggerButton.current?.focus();
    wasConfirming.current = !!preview;
  }, [preview]);
  useEffect(() => { if (message) feedback.current?.focus(); }, [message]);
  const startsAt = session.starts_at ? Date.parse(session.starts_at) : parseSydneyDateTime(`${session.session_date.slice(0, 10)}T${session.start_time}`)?.getTime();
  const closed = !showAction || session.status !== 'open' || now >= Date.parse(session.rsvp_deadline) ||
    (startsAt === undefined || now >= startsAt);
  const coolingDown = nextAllowedAt !== null && now < Date.parse(nextAllowedAt);

  function failureMessage(failure: unknown) {
    const data = (failure as { response?: { data?: { message?: string; next_allowed_at?: string } } })?.response?.data;
    if (data?.next_allowed_at) setNextAllowedAt(data.next_allowed_at);
    setError(true);
    setMessage(data?.message || 'Could not confirm reminders. Try again.');
  }
  async function review() {
    if (inFlight.current) return;
    inFlight.current = true;
    setBusy(true); setMessage(''); setError(false);
    try {
      const result = await api.previewRSVPReminder(session.id);
      setNextAllowedAt(result.next_allowed_at);
      if (result.next_allowed_at && Date.parse(result.next_allowed_at) > Date.now()) {
        setMessage('This session was reminded in the last hour.');
      } else if (result.audience_count === 0) {
        setMessage('Everyone has already responded. No reminders needed.');
      } else {
        requestId.current = crypto.randomUUID();
        setSubmitted(false);
        setPreview(result);
      }
    } catch (failure) { failureMessage(failure); }
    finally { inFlight.current = false; setBusy(false); }
  }
  async function send() {
    if (inFlight.current || !requestId.current) return;
    inFlight.current = true;
    setBusy(true); setSubmitted(true); setError(false); setMessage('');
    try {
      const result = await api.sendRSVPReminder(session.id, requestId.current);
      setNextAllowedAt(result.next_allowed_at);
      setMessage(`${result.queued_count} push reminder${result.queued_count === 1 ? '' : 's'} queued.${result.suppressed_count > 0 ? ` ${result.suppressed_count} could not be queued because notifications are paused or no active push device is registered.` : ''}${result.audience_count === 0 ? ' Everyone has already responded.' : ''}`);
      setPreview(undefined);
      requestId.current = undefined;
      setSubmitted(false);
      triggerButton.current?.focus();
    } catch (failure) {
      const status = (failure as { response?: { status?: number } })?.response?.status;
      // A definitive rejection needs a fresh eligible submission; network errors,
      // timeouts and server failures can still hide a committed result.
      if (status && status >= 400 && status < 500 && status !== 408) setSubmitted(false);
      failureMessage(failure);
    }
    finally { inFlight.current = false; setBusy(false); }
  }
  function cancel() {
    setPreview(undefined); setMessage(''); setError(false); setSubmitted(false);
    requestId.current = undefined;
    triggerButton.current?.focus();
  }
  // Dashboard switches to score entry at session start. Keep an existing
  // confirmation/result alive while hiding the action for fresh reminders.
  if (!showAction && !preview && !message) return null;
  return <div className="border-t border-slate-100 p-4 text-sm sm:p-5">
    {showAction && <button ref={triggerButton} type="button" className="btn-secondary inline-flex min-h-11 items-center gap-2" disabled={busy || closed || coolingDown || !!preview} onClick={() => void review()}>
      <Bell className="h-4 w-4" aria-hidden="true" />{busy && !preview ? 'Checking recipients…' : 'Remind players to RSVP'}
    </button>}
    {closed && <p className="mt-2 text-slate-600">New reminders are available before the RSVP deadline for upcoming open sessions.</p>}
    {coolingDown && <p className="mt-2 text-slate-600">Available again at {formatInTimeZone(nextAllowedAt!, 'Australia/Sydney', "EEE, d MMM 'at' h:mm a")} (Sydney).</p>}
    {preview && <section aria-label="Confirm RSVP reminders" className="mt-3 rounded-lg bg-slate-50 p-3" onKeyDown={event => { if (event.key === 'Escape' && !busy) cancel(); }}>
      <p>Remind {preview.audience_count} {preview.audience_count === 1 ? 'member' : 'members'} to RSVP for {session.title}?</p>
      <p className="mt-1 text-slate-600">Only members without a response are included. The count is checked again when you confirm. Push requires a registered device and notifications to be active.</p>
      <div className="mt-3 flex flex-wrap gap-2">
        <button ref={confirmButton} type="button" className="btn-primary min-h-11" disabled={busy || (!submitted && (closed || coolingDown))} onClick={() => void send()}>{busy ? 'Confirming reminders…' : submitted ? 'Retry confirmation' : 'Confirm reminders'}</button>
        <button type="button" className="btn-secondary min-h-11" disabled={busy} onClick={cancel}>Cancel</button>
      </div>
    </section>}
    {message && <p ref={feedback} tabIndex={-1} role={error ? 'alert' : 'status'} className={`mt-2 ${error ? 'text-red-700' : 'text-slate-600'}`}>{message}</p>}
  </div>;
}
