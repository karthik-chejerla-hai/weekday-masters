import { useEffect, useId, useRef, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { AlertCircle, ArrowUpRight, Check, Loader2, Mic, RotateCcw, X } from 'lucide-react';
import './VoiceDialog.prototype.css';

// Three voice dialog layouts for the existing Ask Rally page. No API calls or microphone access.
export type VoiceDialogPrototypeProps = {
  variant: 'A' | 'B' | 'C';
  phase: 'preparing' | 'waiting' | 'speaking' | 'pausing' | 'processing' | 'review' | 'empty' | 'error';
  transcript: string;
  onTranscriptChange: (text: string) => void;
  onClose: () => void;
  onRetry: () => void;
  onUseText: () => void;
  onFinish: () => void;
  live: boolean;
  audioUrl?: string;
  controls?: ReactNode;
};

const phaseCopy = {
  preparing: { title: 'Starting microphone', detail: 'Allow microphone access if your browser asks.' },
  waiting: { title: 'Speak now', detail: 'Your microphone is on. Start speaking.' },
  speaking: { title: 'Listening', detail: 'Speak naturally. There is no need to press Stop.' },
  pausing: { title: 'Pausing', detail: 'Still listening. Recording ends after a short pause.' },
  processing: { title: 'One moment', detail: 'Preparing your words for review.' },
  review: { title: 'Review your words', detail: 'Make any changes before you use this text.' },
  empty: { title: 'No speech detected', detail: 'Try again when you are ready, or type your message.' },
  error: { title: 'Microphone unavailable', detail: 'Check microphone access in your browser, then try again.' },
};

function SoundMeter({ active }: { active: boolean }) {
  return <div className={`voice-prototype-meter ${active ? 'voice-prototype-meter--active' : ''}`} aria-hidden="true">
    {[8, 15, 27, 19, 37, 24, 43, 30, 18, 32, 15, 8].map((height, index) =>
      <span key={index} style={{ height: active ? height : 5, animationDelay: `${index * -0.09}s` }} />)}
  </div>;
}

function Microphone({ phase, compact = false }: { phase: VoiceDialogPrototypeProps['phase']; compact?: boolean }) {
  const Icon = phase === 'processing' || phase === 'preparing' ? Loader2 : phase === 'review' ? Check : phase === 'error' || phase === 'empty' ? AlertCircle : Mic;
  return <div className={`voice-prototype-mic ${compact ? 'voice-prototype-mic--compact' : ''} voice-prototype-mic--${phase}`} aria-hidden="true">
    <Icon strokeWidth={1.65} />
  </div>;
}

export default function VoiceDialogPrototype({ variant, phase, transcript, onTranscriptChange, onClose, onRetry, onUseText, onFinish, live, audioUrl, controls }: VoiceDialogPrototypeProps) {
  const dialog = useRef<HTMLDialogElement>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const editor = useRef<HTMLTextAreaElement>(null);
  const id = useId();
  const listening = phase === 'waiting' || phase === 'speaking' || phase === 'pausing';
  const review = phase === 'review';
  const recoverable = phase === 'empty' || phase === 'error';
  const copy = phaseCopy[phase];
  const title = live && review ? 'Your recording is ready' : copy.title;
  const detail = live && phase === 'processing' ? 'Preparing your recording for playback.' : live && review ? 'Listen back, or add your message below.' : copy.detail;

  useEffect(() => {
    const element = dialog.current;
    element?.showModal();
    heading.current?.focus();
    return () => { if (element?.open) element.close(); };
  }, []);

  useEffect(() => {
    if (review && !live) editor.current?.focus();
  }, [review, live]);

  const status = <div className="voice-prototype-status" role="status" aria-live="polite" aria-atomic="true">
    <h3>{title}</h3>
    <p>{detail}</p>
  </div>;

  const draft = <div className={`voice-prototype-draft ${review ? 'voice-prototype-draft--review' : ''}`}>
    {review ? <>
      {live && audioUrl && <audio className="voice-prototype-playback" controls src={audioUrl} aria-label="Your recording" />}
      <label htmlFor={`${id}-text`}>{live ? 'Add a message' : 'Your words'}</label>
      <textarea ref={editor} id={`${id}-text`} rows={variant === 'C' ? 3 : 4} value={transcript} onChange={event => onTranscriptChange(event.target.value)} placeholder={live ? 'Type a message to try the review step.' : 'Your words will appear here.'} />
      {live && <p className="voice-prototype-trial-note">Recording stayed on this device. This trial does not transcribe audio.</p>}
    </> : <>
      <p className="voice-prototype-draft-label">{live ? 'Your recording' : 'Sample transcript'}</p>
      <p className={`voice-prototype-words ${!transcript ? 'voice-prototype-words--empty' : ''}`}>
        {live ? 'Your audio stays on this device for playback. Live text is not enabled in this trial.' : transcript || 'Your words will appear here as you speak.'}
        {!live && phase === 'speaking' && <span className="voice-prototype-caret" aria-hidden="true" />}
      </p>
    </>}
  </div>;

  return createPortal(<dialog ref={dialog} className={`voice-dialog-prototype voice-dialog-prototype--${variant}`} aria-labelledby={`${id}-title`} data-phase={phase}
    onCancel={event => { event.preventDefault(); onClose(); }}>
    {variant === 'C' && <div className="voice-prototype-sheet-handle" aria-hidden="true" />}
    <header className="voice-prototype-header">
      <h2 ref={heading} tabIndex={-1} id={`${id}-title`}>Talk to Rally</h2>
      <button className="voice-prototype-close" type="button" aria-label="Close voice dialog" onClick={onClose}><X size={20} /></button>
    </header>

    {variant === 'A' && <div className="voice-prototype-focus-layout">
      <div className="voice-prototype-mic-stage"><Microphone phase={phase} />{listening && <SoundMeter active={phase === 'speaking'} />}</div>
      {status}
      {review && draft}
    </div>}

    {variant === 'B' && <div className="voice-prototype-transcript-layout">
      <div className="voice-prototype-listening-row"><Microphone phase={phase} compact />{status}</div>
      {draft}
      {listening && <div className="voice-prototype-meter-row"><SoundMeter active={phase === 'speaking'} /><span>{phase === 'pausing' ? 'Room for a pause' : 'Speak at your own pace'}</span></div>}
    </div>}

    {variant === 'C' && <div className="voice-prototype-sheet-layout">
      <div className="voice-prototype-sheet-intro"><Microphone phase={phase} compact />{status}</div>
      {listening && <SoundMeter active={phase === 'speaking'} />}
      {(review || (!live && transcript)) && draft}
    </div>}

    <footer className="voice-prototype-footer">
      <div className="voice-prototype-actions">
        <button type="button" className="voice-prototype-button voice-prototype-button--quiet" onClick={onClose}>{recoverable ? 'Type instead' : 'Cancel'}</button>
        {listening && <button type="button" className="voice-prototype-button voice-prototype-button--outline" onClick={onFinish}>Finish now</button>}
        {recoverable && <button type="button" className="voice-prototype-button voice-prototype-button--primary" onClick={onRetry}><RotateCcw size={17} aria-hidden="true" />Try again</button>}
        {review && <>
          <button type="button" className="voice-prototype-button voice-prototype-button--outline" onClick={onRetry}><Mic size={17} aria-hidden="true" />Record again</button>
          <button type="button" className="voice-prototype-button voice-prototype-button--primary" onClick={onUseText} disabled={!transcript.trim()}>Use text<ArrowUpRight size={17} aria-hidden="true" /></button>
        </>}
      </div>
      <p className="voice-prototype-assurance">{review ? 'Nothing is sent or saved until you confirm.' : 'Stops after a pause. You can review before sending.'}</p>
    </footer>
    <aside className="voice-prototype-demo-note" aria-label="Prototype information">{live ? 'Local microphone trial. No audio is sent to Rally.' : 'Prototype preview. Sample words only. No microphone is active.'}</aside>
    {controls && <section className="voice-prototype-controls" aria-label="Prototype controls"><h3>Prototype controls</h3>{controls}</section>}
  </dialog>, document.body);
}
