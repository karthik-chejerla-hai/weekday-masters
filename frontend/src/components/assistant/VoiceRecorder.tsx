import { useEffect, useRef, useState } from 'react';
import { Mic, Square, Loader2 } from 'lucide-react';
import { api } from '../../services/api';
import { assistantError } from './errors';

export default function VoiceRecorder({ onTranscript, disabled = false }: { onTranscript: (text: string) => void; disabled?: boolean }) {
  const [state, setState] = useState<'idle' | 'permission' | 'recording' | 'transcribing'>('idle');
  const [error, setError] = useState('');
  const recorder = useRef<MediaRecorder | null>(null);
  const stream = useRef<MediaStream | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout>>();
  const request = useRef<AbortController | null>(null);
  const generation = useRef(0);
  const supported = typeof MediaRecorder !== 'undefined' && !!navigator.mediaDevices?.getUserMedia;

  const releaseTracks = () => { stream.current?.getTracks().forEach((track) => track.stop()); stream.current = null; };
  const cancel = () => {
    generation.current++;
    clearTimeout(timer.current);
    request.current?.abort();
    if (recorder.current?.state === 'recording') recorder.current.stop();
    releaseTracks();
    setState('idle');
  };
  useEffect(() => () => {
    generation.current++;
    clearTimeout(timer.current);
    request.current?.abort();
    if (recorder.current?.state === 'recording') recorder.current.stop();
    stream.current?.getTracks().forEach((track) => track.stop());
  }, []);

  const start = async () => {
    if (state !== 'idle') return;
    const current = ++generation.current;
    setError(''); setState('permission');
    try {
      const media = await navigator.mediaDevices.getUserMedia({ audio: true });
      if (current !== generation.current) { media.getTracks().forEach((track) => track.stop()); return; }
      stream.current = media;
      const closeMedia = () => {
        media.getTracks().forEach((track) => track.stop());
        if (stream.current === media) stream.current = null;
      };
      const mimeType = ['audio/webm;codecs=opus', 'audio/mp4', 'audio/ogg;codecs=opus'].find((type) => MediaRecorder.isTypeSupported(type));
      if (!mimeType) throw new Error('unsupported');
      const active = new MediaRecorder(media, { mimeType });
      recorder.current = active;
      const chunks: BlobPart[] = [];
      let size = 0;
      active.ondataavailable = (event) => {
        if (current !== generation.current) return;
        if (event.data.size) { chunks.push(event.data); size += event.data.size; }
        if (size > 10 * 1024 * 1024) { cancel(); setError('This recording is too large. Try a shorter one.'); }
      };
      active.onerror = () => { if (current !== generation.current) return; cancel(); setError('Recording failed. Try again or type your request.'); };
      active.onstop = async () => {
        closeMedia();
        if (current !== generation.current) return;
        clearTimeout(timer.current);
        const audio = new Blob(chunks, { type: active.mimeType || mimeType });
        if (!audio.size) { setState('idle'); setError('No audio was recorded. Try again.'); return; }
        setState('transcribing');
        const controller = new AbortController(); request.current = controller;
        try {
          const result = await api.transcribeAudio(audio, controller.signal);
          if (current === generation.current) onTranscript(result.text);
        } catch (err) {
          if (current === generation.current) setError(assistantError(err, 'Could not read the recording. Try again or type your request.'));
        } finally { if (current === generation.current) setState('idle'); }
      };
      active.start(1000); setState('recording');
      timer.current = setTimeout(() => { if (active.state === 'recording') active.stop(); }, 60_000);
    } catch (err) {
      if (current !== generation.current) return;
      releaseTracks();
      const denied = (err as { name?: string }).name === 'NotAllowedError';
      setError(denied ? 'Microphone access was denied. Allow it in your browser or type your request.' : 'Recording is unavailable. Try another browser or type your request.');
      setState('idle');
    }
  };

  return <div className="space-y-2">
    <div className="flex flex-wrap items-center gap-2">
      {state === 'recording' ? <button type="button" className="btn-primary gap-2" onClick={() => recorder.current?.stop()}><Square className="h-4 w-4" /> Stop recording</button> :
        <button type="button" className="btn-secondary gap-2" onClick={start} disabled={disabled || !supported || state !== 'idle'}>
          {state === 'permission' || state === 'transcribing' ? <Loader2 className="h-4 w-4 animate-spin" /> : <Mic className="h-4 w-4" />}
          {state === 'transcribing' ? 'Reading recording…' : state === 'permission' ? 'Waiting for microphone…' : 'Tap to speak'}
        </button>}
      {state !== 'idle' && <button type="button" className="btn-outline" onClick={cancel}>Cancel recording</button>}
      {state === 'recording' && <span role="status" className="text-sm text-red-700">Recording, up to 60 seconds</span>}
    </div>
    {!supported && <p className="text-sm text-slate-600">Voice needs a supported browser on HTTPS or localhost. You can type below.</p>}
    {error && <p role="alert" className="text-sm text-red-700">{error}</p>}
  </div>;
}
