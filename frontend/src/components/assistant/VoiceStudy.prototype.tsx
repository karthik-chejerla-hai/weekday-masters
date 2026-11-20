// Throwaway study: three voice-dialog layouts on /assistant?voice-study=1&variant=A.
import { useCallback, useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ArrowLeft, ArrowRight, Mic, Play, Send } from 'lucide-react';
import VoiceDialogPrototype from './VoiceDialog.prototype';
import { useVoiceTrial } from './useVoiceTrial.prototype';

type Phase = 'preparing' | 'waiting' | 'speaking' | 'pausing' | 'processing' | 'review' | 'empty' | 'error';
type Variant = 'A' | 'B' | 'C';
const sample = 'Alex and Blair beat Casey and Drew, 21 to 17.';
const names = { A: 'Focused microphone', B: 'Words as you speak', C: 'Compact voice sheet' };
const phases: Phase[] = ['preparing', 'waiting', 'speaking', 'pausing', 'processing', 'review', 'empty', 'error'];

export default function VoiceStudyPrototype() {
  const [params, setParams] = useSearchParams();
  const variant: Variant = ['A', 'B', 'C'].includes(params.get('variant') || '') ? params.get('variant') as Variant : 'A';
  const initialPhase = params.get('phase') as Phase;
  const [open, setOpen] = useState(phases.includes(initialPhase));
  const [demoPhase, setDemoPhase] = useState<Phase>(phases.includes(initialPhase) ? initialPhase : 'waiting');
  const [transcript, setTranscript] = useState(initialPhase === 'review' || initialPhase === 'speaking' ? sample : '');
  const [draft, setDraft] = useState('');
  const [live, setLive] = useState(false);
  const [notice, setNotice] = useState('');
  const timers = useRef<ReturnType<typeof setTimeout>[]>([]);
  const microphoneButton = useRef<HTMLButtonElement>(null);
  const trial = useVoiceTrial();
  const phase = live ? trial.phase : demoPhase;
  const capture = params.get('capture') === '1';

  const clearDemo = useCallback(() => { timers.current.forEach(clearTimeout); timers.current = []; }, []);
  useEffect(() => clearDemo, [clearDemo]);
  const selectVariant = useCallback((direction: number) => {
    const choices: Variant[] = ['A', 'B', 'C'];
    const next = choices[(choices.indexOf(variant) + direction + 3) % 3];
    setParams((current) => { current.set('variant', next); return current; }, { replace: true });
  }, [setParams, variant]);
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if ((event.target as HTMLElement).closest('input,textarea,select,[contenteditable=true]')) return;
      if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
        event.preventDefault(); selectVariant(event.key === 'ArrowLeft' ? -1 : 1);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [selectVariant]);

  const close = () => {
    clearDemo(); trial.cancel(); setOpen(false); setLive(false);
    requestAnimationFrame(() => microphoneButton.current?.focus());
  };
  const preview = () => { clearDemo(); trial.cancel(); setLive(false); setTranscript(''); setDemoPhase('waiting'); setOpen(true); };
  const runSample = () => {
    preview();
    const later = (ms: number, action: () => void) => timers.current.push(setTimeout(action, ms));
    later(1200, () => { setDemoPhase('speaking'); setTranscript('Alex and Blair'); });
    later(2500, () => setTranscript('Alex and Blair beat Casey and Drew,'));
    later(3500, () => setDemoPhase('pausing'));
    later(4700, () => { setDemoPhase('speaking'); setTranscript(sample); });
    later(6100, () => setDemoPhase('pausing'));
    later(9100, () => setDemoPhase('processing'));
    later(10100, () => setDemoPhase('review'));
  };
  const startTrial = () => {
    clearDemo(); setLive(true); setTranscript(''); setOpen(true); void trial.start();
  };
  const controls = !capture ? <div className="voice-study-controls">
    <div className="voice-study-variants">
      <button type="button" aria-label="Previous design" onClick={() => selectVariant(-1)}><ArrowLeft size={17} /></button>
      <span>{variant}. {names[variant]}</span>
      <button type="button" aria-label="Next design" onClick={() => selectVariant(1)}><ArrowRight size={17} /></button>
    </div>
    <div className="voice-study-stage">
      <label>State <select aria-label="Preview state" value={phase} disabled={live} onChange={(event) => {
        clearDemo(); setDemoPhase(event.target.value as Phase);
        setTranscript(['review', 'speaking', 'pausing'].includes(event.target.value) ? sample : '');
      }}>{phases.map((p) => <option key={p}>{p}</option>)}</select></label>
      <button type="button" onClick={runSample}><Play size={14} /> Run sample</button>
    </div>
  </div> : undefined;

  return <div className="mx-auto max-w-2xl space-y-6 pb-24">
    <header className="page-heading"><p className="page-kicker">Your club assistant</p><h1 className="page-title">Ask Rally</h1><p className="mt-2 text-sm text-slate-600">Ask questions or record scores and expenses. Review changes before saving.</p></header>
    <section className="card p-6 space-y-4" aria-label="Voice study composer">
      <label className="label" htmlFor="study-draft">Your message</label>
      <textarea id="study-draft" value={draft} onChange={(e) => setDraft(e.target.value)} rows={3} className="input" placeholder="How much court credit do we have?" />
      <div className="flex justify-between gap-3"><button ref={microphoneButton} className="btn-outline gap-2" onClick={preview}><Mic size={18} /> Speak to Rally</button><button className="btn-primary gap-2" disabled={!draft.trim()} onClick={() => setNotice('Prototype only. Nothing was sent or saved.')}><Send size={17} /> Send</button></div>
      <p className="text-xs text-slate-600">Review the words before sending.</p>
      {notice && <p role="status" className="text-sm text-primary-800">{notice}</p>}
    </section>
    <section className="rounded-xl border border-dashed border-slate-300 p-5 text-sm space-y-4" aria-label="Prototype laboratory">
      <div><h2 className="font-semibold text-slate-900">Voice dialog study</h2><p className="mt-1 text-slate-600">Three layouts. Sample words are simulated. No messages or scores are saved.</p></div>
      {controls}
      <div className="flex flex-wrap gap-2"><button className="btn-outline gap-2" onClick={preview}>Open dialog</button><button className="btn-ghost gap-2" onClick={runSample}><Play size={16} /> Play sample flow</button></div>
      <div className="border-t border-slate-200 pt-4"><h3 className="font-semibold">Try automatic stopping</h3><p className="mt-1 text-slate-600">This local trial detects speech with Silero. It stops after three seconds of silence. Your recording stays in this browser for replay. It is not uploaded or transcribed.</p><button className="btn-outline mt-3 gap-2" onClick={startTrial}><Mic size={16} /> Try microphone locally</button></div>
      {trial.error && <p role="alert" className="text-red-700">{trial.error}</p>}
    </section>
    {!open && !capture && <div className="voice-study-floating">{controls}</div>}
    {open && <VoiceDialogPrototype variant={variant} phase={phase} transcript={transcript} onTranscriptChange={setTranscript} onClose={close} onRetry={live ? startTrial : runSample} onUseText={() => { setDraft(transcript); close(); }} onFinish={() => { if (live) trial.finish(); else { clearDemo(); setTranscript(sample); setDemoPhase('review'); } }} live={live} audioUrl={trial.audioUrl || undefined} controls={controls} />}
  </div>;
}
