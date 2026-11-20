import { useEffect, useRef, useState } from 'react';
import type { MicVAD } from '@ricky0123/vad-web';

export type VoiceTrialPhase = 'preparing' | 'waiting' | 'speaking' | 'pausing' | 'processing' | 'review' | 'empty' | 'error';

type Trial = {
  stream?: MediaStream;
  context?: AudioContext;
  detector?: MicVAD;
  recorder?: MediaRecorder;
  timers: ReturnType<typeof setTimeout>[];
  noSpeechTimer?: ReturnType<typeof setTimeout>;
  validSpeech: boolean;
  finishing: boolean;
  initialising: boolean;
  released: boolean;
  processorReleased: boolean;
};

function release(trial: Trial) {
  if (!trial.released) {
    trial.released = true;
    trial.timers.forEach(clearTimeout);
    if (trial.recorder) {
      trial.recorder.ondataavailable = null;
      trial.recorder.onstop = null;
      trial.recorder.onerror = null;
      if (trial.recorder.state !== 'inactive') trial.recorder.stop();
    }
    trial.stream?.getTracks().forEach((track) => track.stop());
  }
  // This library cannot destroy an uninitialised detector. Finish setup with
  // the stopped stream, then release its model and the context we own.
  if (trial.initialising || trial.processorReleased) return;
  trial.processorReleased = true;
  void (async () => {
    try { await trial.detector?.destroy(); } catch { /* Setup may have failed before audio nodes existed. */ }
    finally {
      if (trial.context && trial.context.state !== 'closed') await trial.context.close().catch(() => {});
    }
  })();
}

export function useVoiceTrial() {
  const [phase, setPhase] = useState<VoiceTrialPhase>('waiting');
  const [audioUrl, setAudioUrl] = useState<string>();
  const [error, setError] = useState('');
  const active = useRef<Trial | null>(null);
  const replay = useRef<string>();

  const clearReplay = () => {
    if (replay.current) URL.revokeObjectURL(replay.current);
    replay.current = undefined;
    setAudioUrl(undefined);
  };

  const fail = (trial: Trial, message: string) => {
    if (active.current !== trial) return;
    active.current = null;
    release(trial);
    setError(message);
    setPhase('error');
  };

  const finishTrial = (trial: Trial) => {
    if (active.current !== trial || trial.finishing) return;
    trial.finishing = true;
    trial.timers.forEach(clearTimeout);
    if (!trial.validSpeech || !trial.recorder) {
      active.current = null;
      release(trial);
      setPhase('empty');
      return;
    }
    setPhase('processing');
    if (trial.recorder.state !== 'inactive') trial.recorder.stop();
  };

  const cancel = () => {
    const trial = active.current;
    active.current = null;
    if (trial) release(trial);
    clearReplay();
    setError('');
    setPhase('waiting');
  };

  const start = async () => {
    if (active.current) return;
    clearReplay();
    setError('');
    setPhase('preparing');
    const trial: Trial = {
      timers: [], validSpeech: false, finishing: false,
      initialising: false, released: false, processorReleased: false,
    };
    active.current = trial;
    const listening = () => active.current === trial && !trial.finishing;
    try {
      if (!navigator.mediaDevices?.getUserMedia || typeof MediaRecorder === 'undefined' || typeof AudioContext === 'undefined') {
        throw new Error('This browser cannot record here. Use a supported browser on HTTPS or localhost.');
      }
      const mimeType = ['audio/webm;codecs=opus', 'audio/mp4', 'audio/ogg;codecs=opus']
        .find((type) => MediaRecorder.isTypeSupported(type));
      if (!mimeType) throw new Error('This browser does not support a recording format for this trial.');
      // Resume during the user's action, before loading the model.
      trial.context = new AudioContext();
      await trial.context.resume();
      if (!listening()) return;
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: { channelCount: 1, echoCancellation: true, noiseSuppression: true, autoGainControl: true },
      });
      if (!listening()) { stream.getTracks().forEach((track) => track.stop()); return; }
      trial.stream = stream;
      const recorder = new MediaRecorder(stream, { mimeType });
      trial.recorder = recorder;
      const { MicVAD } = await import('@ricky0123/vad-web');
      if (!listening()) return;
      trial.initialising = true;
      trial.detector = await MicVAD.new({
        model: 'v5', startOnLoad: false,
        baseAssetPath: '/prototype-voice-assets/', onnxWASMBasePath: '/prototype-voice-assets/',
        audioContext: trial.context,
        getStream: async () => stream,
        pauseStream: async () => {}, // The recorder owns the shared stream.
        resumeStream: async () => stream,
        positiveSpeechThreshold: 0.5, negativeSpeechThreshold: 0.35,
        redemptionMs: 3000, minSpeechMs: 400, submitUserSpeechOnPause: false,
        onSpeechStart: () => { if (listening()) setPhase('speaking'); },
        onSpeechRealStart: () => {
          if (!listening()) return;
          trial.validSpeech = true;
          clearTimeout(trial.noSpeechTimer);
          setPhase('speaking');
        },
        onFrameProcessed: ({ isSpeech }) => {
          if (!listening() || !trial.validSpeech) return;
          if (isSpeech >= 0.5) setPhase('speaking');
          else if (isSpeech < 0.35) setPhase('pausing');
        },
        onVADMisfire: () => { if (listening()) setPhase('waiting'); },
        onSpeechEnd: () => finishTrial(trial),
      });
      if (listening()) {
        const chunks: BlobPart[] = [];
        let size = 0;
        recorder.ondataavailable = ({ data }) => {
          if (active.current !== trial) return;
          size += data.size;
          if (size > 10 * 1024 * 1024) { fail(trial, 'This recording is too large. Try a shorter one.'); return; }
          if (data.size) chunks.push(data);
        };
        recorder.onerror = () => fail(trial, 'Recording failed. Please try again.');
        recorder.onstop = () => {
          if (active.current !== trial) return;
          const audio = new Blob(chunks, { type: recorder.mimeType || mimeType });
          active.current = null;
          release(trial);
          if (!trial.validSpeech || !audio.size) { setPhase('empty'); return; }
          replay.current = URL.createObjectURL(audio);
          setAudioUrl(replay.current);
          setPhase('review');
        };
        recorder.start(250);
        trial.timers.push(setTimeout(() => finishTrial(trial), 60_000));
      }
      // Also complete setup after cancellation, using only the stopped stream.
      await trial.detector.start();
      trial.initialising = false;
      if (!listening()) { release(trial); return; }
      if (!trial.validSpeech) setPhase('waiting');
      trial.noSpeechTimer = setTimeout(() => { if (!trial.validSpeech) finishTrial(trial); }, 8000);
      trial.timers.push(trial.noSpeechTimer);
    } catch (reason) {
      trial.initialising = false;
      const denied = (reason as { name?: string }).name === 'NotAllowedError';
      fail(trial, denied ? 'Microphone access was denied. Allow access in your browser and try again.'
        : reason instanceof Error ? reason.message : 'Could not start the microphone. Please try again.');
      release(trial);
    }
  };

  useEffect(() => () => {
    const trial = active.current;
    active.current = null;
    if (trial) release(trial);
    if (replay.current) URL.revokeObjectURL(replay.current);
  }, []);

  return { phase, audioUrl, error, start, cancel, finish: () => { if (active.current) finishTrial(active.current); } };
}
