import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { Loader2, Send } from 'lucide-react';
import { useAuth } from '../../context/useAuth';
import { api } from '../../services/api';
import type { AssistantMessage, ExpensePreview, GamePreview } from '../../types';
import VoiceRecorder from './VoiceRecorder';
import ExpenseReview from './ExpenseReview';
import GameForm from '../games/GameForm';
import { assistantError } from './errors';

export default function AssistantPanel({ sessionId, mode, onGameSaved }: { sessionId?: string; mode?: 'games'; onGameSaved?: () => void }) {
  const { isAdmin } = useAuth();
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [messages, setMessages] = useState<AssistantMessage[]>([]);
  const [text, setText] = useState('');
  const [busy, setBusy] = useState(false);
  const [savingGame, setSavingGame] = useState(false);
  const [error, setError] = useState('');
  const [expense, setExpense] = useState<ExpensePreview | null>(null);
  const [game, setGame] = useState<GamePreview | null>(null);
  const messageInput = useRef<HTMLTextAreaElement | null>(null);
  const request = useRef<AbortController | null>(null);
  const locked = useRef(false);
  const gameSaving = useRef(false);
  const gameReview = useRef<HTMLElement | null>(null);
  useEffect(() => { if (game) gameReview.current?.scrollIntoView?.({ behavior: 'smooth', block: 'start' }); }, [game]);

  useEffect(() => {
    let cancelled = false;
    api.assistantStatus().then((status) => { if (!cancelled) setEnabled(status.enabled); }).catch(() => {
      if (!cancelled) { setEnabled(false); setError('Could not check assistant availability. Refresh this page to try again.'); }
    });
    return () => { cancelled = true; request.current?.abort(); };
  }, []);

  const handleGameSaving = (saving: boolean) => {
    gameSaving.current = saving;
    setSavingGame(saving);
  };

  const send = async (value = text) => {
    if (!value.trim() || locked.current || gameSaving.current || !enabled) return;
    locked.current = true;
    const controller = new AbortController(); request.current = controller;
    const next: AssistantMessage[] = [...messages.slice(-14), { role: 'user', content: value.trim() }];
    setBusy(true); setError(''); setExpense(null); setGame(null);
    try {
      const reply = await api.askAssistant(next, sessionId, controller.signal);
      if (controller.signal.aborted) return;
      setMessages([...next, { role: 'assistant', content: reply.message }]);
      setText(''); setExpense(reply.expense ?? null); setGame(reply.game ?? null);
    } catch (err) {
      if (!controller.signal.aborted) setError(assistantError(err, 'Could not reach the assistant. Try again or use the form.'));
    } finally { locked.current = false; if (!controller.signal.aborted) setBusy(false); }
  };

  return <div className="space-y-5">
    <section className="card p-5 sm:p-6 space-y-5" aria-label="Ask Rally">
      {mode === 'games' && <div><h2 className="text-lg font-semibold text-slate-950">Say the score</h2><p className="mt-1 text-sm text-slate-600">Say both teams and their scores. For example: Alice and Bob beat Cara and Dan, 21 to 17.</p></div>}
      {enabled === false && <p role="status" className="rounded-xl bg-amber-50 p-3 text-sm text-amber-900">The assistant is unavailable. {mode === 'games' ? 'Use the score form below.' : isAdmin ? 'You can still record an expense with the form.' : 'You can view sessions and balances from the menu.'}</p>}
      {messages.length > 0 && <div role="log" aria-label="Conversation" className="max-h-96 space-y-3 overflow-y-auto">
        {messages.map((message, index) => <div key={index} className={`rounded-xl p-3 text-sm whitespace-pre-wrap ${message.role === 'user' ? 'ml-6 bg-primary-50 text-primary-950' : 'mr-6 bg-slate-50 text-slate-800'}`}><p className="mb-1 text-xs font-semibold text-slate-500">{message.role === 'user' ? 'You' : 'Rally'}</p>{message.content}</div>)}
      </div>}
      <form onSubmit={(event) => { event.preventDefault(); void send(); }} className="space-y-3">
        <div className="rounded-xl border border-slate-300 bg-white p-3 focus-within:border-primary-600">
          <label htmlFor="assistant-message" className="label">Your message</label>
          <textarea ref={messageInput} id="assistant-message" rows={2} className="input min-h-20 resize-y" maxLength={4000} value={text} disabled={busy || savingGame} aria-describedby="assistant-privacy" onChange={(event) => { setText(event.target.value); setExpense(null); setGame(null); }} placeholder={mode === 'games' ? 'Alice and Bob beat Cara and Dan, 21 to 17.' : sessionId && isAdmin ? 'We played three hours and used eight shuttles.' : 'How much court credit do we have?'} />
          <div className="mt-3 flex flex-wrap items-start justify-between gap-2">
            <div className="min-w-0 flex-1">
              <VoiceRecorder disabled={!enabled || busy || savingGame} onTranscript={(value) => {
                setText(value);
                // Keep late transcripts without replacing a saving review or retry identity.
                if (gameSaving.current) return;
                setExpense(null); setGame(null);
                if (mode === 'games') void send(value);
                else messageInput.current?.focus();
              }} />
            </div>
            <button type="submit" className="btn-primary gap-2" aria-label={busy ? 'Working on your request…' : 'Send message'} disabled={!enabled || busy || savingGame || !text.trim()}>{busy ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" /> : <Send className="h-4 w-4" aria-hidden="true" />}{busy ? 'Working…' : 'Send'}</button>
          </div>
        </div>
        {error && <p role="alert" className="text-sm text-red-700">{error}</p>}
      </form>
      {mode !== 'games' && messages.length === 0 && <div className="flex flex-wrap gap-2" role="group" aria-label="Example prompts">
        {[
          { label: 'Court credit', prompt: 'How much court credit do we have?' },
          { label: 'Record a score', prompt: 'Help me record a doubles game score.' },
          isAdmin ? { label: 'Session expense', prompt: 'We played three hours and used eight shuttles.' } : { label: 'Next session', prompt: 'When is our next session?' },
        ].map(({ label, prompt }) => <button key={label} type="button" disabled={busy || savingGame} className="btn-ghost min-h-11 border border-slate-200 text-sm" onClick={() => { setText(prompt); setExpense(null); setGame(null); messageInput.current?.focus(); }}>{label}</button>)}
      </div>}
      <p id="assistant-privacy" className="text-xs leading-5 text-slate-600">{mode === 'games' ? 'Review the teams and score before saving.' : 'Review the words before sending.'} Recordings are processed for this request and are not saved by Rally.</p>
      {isAdmin && mode !== 'games' && <Link className="inline-flex min-h-11 items-center text-sm font-semibold text-primary-700 underline underline-offset-4" to={sessionId ? `/admin/sessions/${sessionId}/expense` : '/sessions'}>{sessionId ? 'Use the expense form' : 'Choose a session to expense'}</Link>}

    </section>
    {game && <section ref={gameReview} className="card scroll-mt-4 p-5 space-y-4" aria-label="Review game"><div><h2 className="text-lg font-semibold">Review game</h2><p className="text-sm text-slate-600">{game.session.title}</p></div><GameForm key={JSON.stringify(game)} sessionId={game.session.id} initial={game} onSavingChange={handleGameSaving} onSaved={() => { setGame(null); setMessages((old) => [...old, { role: 'assistant', content: 'Game saved.' }]); onGameSaved?.(); }} onCancel={() => setGame(null)} /></section>}
    {expense && isAdmin && <ExpenseReview key={expense.session.id} preview={expense} onRefresh={setExpense} />}
  </div>;
}
