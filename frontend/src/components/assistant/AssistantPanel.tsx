import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { Loader2, Send, MessageCircle } from 'lucide-react';
import { useAuth } from '../../context/useAuth';
import { api } from '../../services/api';
import type { AssistantMessage, ExpensePreview } from '../../types';
import VoiceRecorder from './VoiceRecorder';
import ExpenseReview from './ExpenseReview';
import { assistantError } from './errors';

export default function AssistantPanel({ sessionId }: { sessionId?: string }) {
  const { isAdmin } = useAuth();
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [messages, setMessages] = useState<AssistantMessage[]>([]);
  const [text, setText] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [expense, setExpense] = useState<ExpensePreview | null>(null);
  const request = useRef<AbortController | null>(null);
  const locked = useRef(false);

  useEffect(() => {
    let cancelled = false;
    api.assistantStatus().then((status) => { if (!cancelled) setEnabled(status.enabled); }).catch(() => {
      if (!cancelled) { setEnabled(false); setError('Could not check assistant availability. Refresh this page to try again.'); }
    });
    return () => { cancelled = true; request.current?.abort(); };
  }, []);

  const send = async () => {
    if (!text.trim() || locked.current || !enabled) return;
    locked.current = true;
    const controller = new AbortController(); request.current = controller;
    const next: AssistantMessage[] = [...messages.slice(-14), { role: 'user', content: text.trim() }];
    setBusy(true); setError(''); setExpense(null);
    try {
      const reply = await api.askAssistant(next, sessionId, controller.signal);
      if (controller.signal.aborted) return;
      setMessages([...next, { role: 'assistant', content: reply.message }]);
      setText(''); setExpense(reply.expense ?? null);
    } catch (err) {
      if (!controller.signal.aborted) setError(assistantError(err, 'Could not reach the assistant. Try again or use the form.'));
    } finally { locked.current = false; if (!controller.signal.aborted) setBusy(false); }
  };

  return <div className="space-y-5">
    <section className="card p-5 sm:p-6 space-y-5" aria-label="Ask Rally">
      <div className="flex items-start gap-3"><span className="rounded-2xl bg-primary-100 p-3 text-primary-800"><MessageCircle className="h-6 w-6" /></span><div>
        <h2 className="text-lg font-semibold text-slate-950">Tell Rally what happened</h2>
        <p className="mt-1 text-sm text-slate-600">{sessionId && isAdmin ? 'Say how long you played and how many shuttles you used.' : 'Ask about sessions, player balances or club supplies.'}</p>
      </div></div>
      {enabled === false && <p role="status" className="rounded-xl bg-amber-50 p-3 text-sm text-amber-900">The assistant is unavailable. {isAdmin ? 'You can still record an expense with the form.' : 'You can view sessions and balances from the menu.'}</p>}
      {messages.length > 0 && <div role="log" aria-label="Conversation" className="max-h-96 space-y-3 overflow-y-auto">
        {messages.map((message, index) => <div key={index} className={`rounded-xl p-3 text-sm whitespace-pre-wrap ${message.role === 'user' ? 'ml-6 bg-primary-50 text-primary-950' : 'mr-6 bg-slate-50 text-slate-800'}`}><p className="mb-1 text-xs font-semibold text-slate-500">{message.role === 'user' ? 'You' : 'Rally'}</p>{message.content}</div>)}
      </div>}
      <VoiceRecorder disabled={!enabled || busy} onTranscript={(value) => { setText(value); setExpense(null); }} />
      <form onSubmit={(event) => { event.preventDefault(); void send(); }} className="space-y-3">
        <div><label htmlFor="assistant-message" className="label">Your message</label>
          <textarea id="assistant-message" className="input min-h-28 resize-y" maxLength={4000} value={text} disabled={busy} onChange={(event) => { setText(event.target.value); setExpense(null); }} placeholder={sessionId && isAdmin ? 'We played three hours and used eight shuttles.' : 'How much court credit do we have?'} />
          <p className="mt-2 text-xs text-slate-500">Review the words before sending. Recordings are processed for this request and are not saved by Rally.</p>
        </div>
        {error && <p role="alert" className="text-sm text-red-700">{error}</p>}
        <button type="submit" className="btn-primary w-full gap-2" disabled={!enabled || busy || !text.trim()}>{busy ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}{busy ? 'Working on your request…' : 'Send message'}</button>
      </form>
      {isAdmin && <Link className="btn-outline w-full" to={sessionId ? `/admin/sessions/${sessionId}/expense` : '/sessions'}>{sessionId ? 'Use the expense form' : 'Choose a session to expense'}</Link>}
    </section>
    {expense && isAdmin && <ExpenseReview key={expense.session.id} preview={expense} onRefresh={setExpense} />}
  </div>;
}
