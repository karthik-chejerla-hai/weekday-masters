import { useEffect, useRef, useState } from 'react';
import { ExternalLink, Loader2, Mail, Send, X } from 'lucide-react';
import { formatInTimeZone } from 'date-fns-tz';
import { api } from '../../services/api';
import type { InvitationDelivery, InvitationPreview, User } from '../../types';
import { displayName } from '../../utils/members';

function message(error: unknown): string {
  return (error as { response?: { data?: { error?: string } } })?.response?.data?.error
    || 'The request did not finish. Use Check result to retrieve its status without sending another email.';
}

export default function InvitationPanel({ member, onClose }: { member: User; onClose: () => void }) {
  const [preview, setPreview] = useState<InvitationPreview | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<InvitationDelivery | null>(null);
  const [narrow, setNarrow] = useState(false);
  const [uncertain, setUncertain] = useState(false);
  const attempt = useRef<{ id: string; test: boolean } | null>(null);
  const heading = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    let active = true;
    api.previewInvitation(member.id).then((data) => { if (active) setPreview(data); })
      .catch((err: unknown) => { if (active) setError(message(err)); });
    heading.current?.focus();
    return () => { active = false; };
  }, [member.id]);

  const send = async (test: boolean, retry = false) => {
    if (!preview || busy) return;
    setBusy(true);
    setError(null);
    if (!retry || !attempt.current) attempt.current = { id: crypto.randomUUID(), test };
    try {
      const delivery = await api.sendInvitation(member.id, attempt.current.id, preview.recipient_email, attempt.current.test);
      setResult(delivery);
      setUncertain(false);
      setPreview((current) => current && ({ ...current, [delivery.is_test ? 'last_test' : 'last_invitation']: delivery }));
      // Pending means the original request is still running. Keep its ID.
      setUncertain(delivery.status === 'pending');
    } catch (err) {
      setError(message(err));
      const status = (err as { response?: { status?: number } })?.response?.status;
      setUncertain(!status || status >= 500);
    } finally { setBusy(false); }
  };

  return (
    <section aria-labelledby={`invitation-heading-${member.id}`} className="mt-4 rounded-xl border border-primary-100 bg-slate-50 p-4 sm:p-5">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 id={`invitation-heading-${member.id}`} ref={heading} tabIndex={-1} className="font-semibold text-slate-900 focus:outline-none">Invitation for {displayName(member)}</h2>
          <p className="mt-1 text-sm text-slate-600">Review the email before sending. Opening this preview sends nothing.</p>
        </div>
        <button type="button" onClick={onClose} disabled={busy} aria-label="Close invitation preview" className="rounded-lg p-2 text-slate-500 hover:bg-slate-200"><X className="h-5 w-5" /></button>
      </div>
      {error && <p role="alert" className="mt-3 rounded-lg bg-red-50 p-3 text-sm text-red-700">{error}</p>}
      {!preview && !error && <p role="status" className="mt-4 text-sm text-slate-500">Loading email preview…</p>}
      {preview && <>
        <dl className="my-4 grid gap-2 break-words text-sm sm:grid-cols-[5rem_1fr]">
          <dt className="text-slate-500">To</dt><dd className="font-medium text-slate-900">{preview.recipient_email}</dd>
          <dt className="text-slate-500">From</dt><dd>{preview.from_email || 'Not configured'}</dd>
          <dt className="text-slate-500">Subject</dt><dd>{preview.subject}</dd>
        </dl>
        <div className="mb-3 flex flex-wrap items-center justify-between gap-3 text-sm">
          <div className="flex gap-2" aria-label="Email preview width">
            <button type="button" aria-pressed={!narrow} onClick={() => setNarrow(false)} className={`rounded-lg px-3 py-1.5 ${!narrow ? 'bg-primary-100 text-primary-800' : 'bg-white text-slate-600'}`}>Desktop</button>
            <button type="button" aria-pressed={narrow} onClick={() => setNarrow(true)} className={`rounded-lg px-3 py-1.5 ${narrow ? 'bg-primary-100 text-primary-800' : 'bg-white text-slate-600'}`}>Mobile</button>
          </div>
          <a href={preview.login_url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-primary-700 underline">Open sign-in link<ExternalLink className="h-3.5 w-3.5" /></a>
        </div>
        <iframe title={`Invitation email for ${displayName(member)}`} srcDoc={preview.html} sandbox="" className={`mx-auto block h-[640px] w-full rounded-lg border border-slate-200 bg-white ${narrow ? 'max-w-[360px]' : 'max-w-[640px]'}`} />
        <p className="mt-2 text-xs text-slate-500">This is the email content. Email apps can display fonts and spacing differently. Use a test email to check your inbox.</p>
        <div className="mt-5 grid gap-4 lg:grid-cols-2">
          <div className="rounded-xl border border-slate-200 bg-white p-4">
            <h3 className="font-medium text-slate-900">1. Test the email</h3>
            <p className="mt-1 break-words text-sm text-slate-600">Send a copy to <strong>{preview.test_recipient}</strong>. The subject starts with [TEST]. {displayName(member)} will not receive it.</p>
            {preview.test_blocked_reason && <p className="mt-2 text-sm text-amber-800">{preview.test_blocked_reason}</p>}
            <button type="button" disabled={busy || uncertain || !preview.can_test} onClick={() => send(true)} className="btn-secondary mt-3 inline-flex items-center gap-2 disabled:opacity-50"><Mail className="h-4 w-4" />Send test to me</button>
            {preview.last_test && <DeliveryStatus delivery={preview.last_test} />}
          </div>
          <div className="rounded-xl border border-slate-200 bg-white p-4">
            <h3 className="font-medium text-slate-900">2. Send the invitation</h3>
            <p className="mt-1 break-words text-sm text-slate-600">Send this email to <strong>{preview.recipient_email}</strong>.</p>
            {preview.send_blocked_reason && <p className="mt-2 text-sm text-amber-800">{preview.send_blocked_reason}</p>}
            <button type="button" disabled={busy || uncertain || !preview.can_send} onClick={() => send(false)} className="btn-primary mt-3 inline-flex items-center gap-2 disabled:opacity-50"><Send className="h-4 w-4" />{preview.last_invitation ? 'Resend invitation' : 'Send invitation'}</button>
            {preview.last_invitation && <DeliveryStatus delivery={preview.last_invitation} />}
          </div>
        </div>
        {busy && <p role="status" className="mt-3 flex items-center gap-2 text-sm text-slate-600"><Loader2 className="h-4 w-4 animate-spin" />Sending email…</p>}
        {result && !busy && <p role="status" className="mt-3 text-sm text-slate-700">{result.is_test ? 'Test email' : 'Invitation'}: {result.message}</p>}
        {uncertain && <button type="button" onClick={() => send(attempt.current?.test ?? true, true)} disabled={busy} className="btn-secondary mt-3">Check result</button>}
        <p className="mt-4 text-xs leading-5 text-slate-500">The sign-in link does not grant access by itself. Members must use their invited Google account. Test copies do not sign in as the member or change their record.</p>
      </>}
    </section>
  );
}

function DeliveryStatus({ delivery }: { delivery: InvitationDelivery }) {
  const label = { accepted: 'Accepted for delivery', pending: 'Result pending', failed: 'Send failed', unknown: 'Result unknown' }[delivery.status];
  return <p className="mt-3 text-xs text-slate-500">{label} · {formatInTimeZone(delivery.accepted_at || delivery.created_at, 'Australia/Sydney', 'd MMM yyyy, h:mm a')}. {delivery.status !== 'accepted' && delivery.message}</p>;
}
