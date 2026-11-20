# Voice dialog spike

Throwaway experiment for PR #121 and issue #66. This branch is evidence for a
design decision, not a production implementation.

## Question and result

Can a clear microphone dialog remove the need to press Stop, while keeping a
review step and avoiding a new paid speech service?

Yes for the tested synthetic speech. A local Silero speech detector stopped the
actual MediaRecorder about three seconds after speech ended. It retained both
sentences across a 1.4-second pause. This is a limited desktop Chromium result,
not evidence of reliable endpoint detection in a badminton hall or on a phone.

The proposed direction is layout A (large microphone, then editable review) on
desktop and layout C (bottom sheet) on mobile. The waiting and speaking states
have distinct text and microphone appearance. Automatic stopping prepares a
draft. It never sends a message or saves a game or expense.

## Alternatives assessed by three subagents

| Approach | Benefit | Limit | Verdict |
| --- | --- | --- | --- |
| Local Silero detection, then existing batch transcription | Automatic stopping; no new detection service fee; works independently of live-text support | Runtime/model download; silence threshold needs real-device and hall trials; existing transcription costs remain | Preferred first experiment |
| Browser SpeechRecognition with interim results | Live words without a Rally speech-service account | Limited browser support; browser service may receive audio; errors and service disconnects need fallback | Optional enhancement, not the sole recording path |
| Browser-only Whisper | Local text with no per-request inference bill | Larger model downloads, CPU/battery load and device-dependent speed | Defer until measured on target phones |

Simple amplitude detection was also considered. It can classify racket impacts
and other loud sounds as speech. Speech detection is a better candidate, but it
can still detect other people speaking. A manual Finish now control remains an
optional escape when background speech prevents automatic completion.

Layout B shows the optional live-text concept. Its words are simulated. This
spike does not invoke browser SpeechRecognition or transcribe audio. Chrome
capability presence is not evidence that a language pack is installed or that
recognition works on the user's devices.

## Run and inspect

From this branch's frontend directory, run `npm ci`, then `npm run prototype:voice`.
Open <http://127.0.0.1:4175/assistant?voice-study=1&variant=A>.

Use the arrow controls or left/right keys to compare A, B and C. Run sample shows
the proposed state sequence using sample words. Try microphone locally uses a
real microphone, local speech detection and browser-only audio replay. It does
not upload or transcribe audio. API responses are synthetic and no backend proxy
is configured. The trial uses a three-second end pause, an eight-second initial
silence limit, a sixty-second recording limit and a ten-megabyte audio limit.

The generated model/runtime files are ignored. Screenshots and generated test
audio remain outside the repository. No production route or provider contract
was changed on the implementation branch.

## Evidence

- Desktop Chromium with generated speech: both sentences retained across a
  1.4-second pause; recording ended automatically about three seconds after the
  last speech frame; non-empty local replay available.
- Initial silence: empty state after eight seconds, microphone track stopped.
- Cancel during speech: dialog closed, microphone track stopped, no late review.
- All three audio cases: zero requests outside localhost and no browser errors.
- Desktop, 390px and 320px layouts: no horizontal overflow; A/B centred, C at the
  bottom; Escape closes the dialog; variant controls work.
- Review found and fixed host spacing overriding native dialog positioning and
  model initialization before recording-format validation. The second technical
  review found no blocker, major or minor issue in the correction.

Remaining limits: physical iPhone/Android testing, actual hall noise, names and
score recognition accuracy, pause duration preference, permission prompts,
background/tab interruption, and partial VAD initialization failures. The VAD
library can fail cleanup before its audio nodes are initialized; this experiment
does not claim complete model cleanup on every library startup error.

## Primary references

- [Silero VAD](https://github.com/snakers4/silero-vad)
- [Browser VAD API](https://docs.vad.ricky0123.com/user-guide/api/)
- [SpeechRecognition](https://developer.mozilla.org/en-US/docs/Web/API/SpeechRecognition)
- [Interim results](https://developer.mozilla.org/en-US/docs/Web/API/SpeechRecognition/interimResults)
- [Local recognition](https://developer.mozilla.org/en-US/docs/Web/API/SpeechRecognition/processLocally)
- [Chrome 139 local speech recognition](https://developer.chrome.com/release-notes/139)
- [Safari speech recognition](https://webkit.org/blog/11648/new-webkit-features-in-safari-14-1/)
- [Transformers.js](https://huggingface.co/docs/transformers.js/en/index)
- [Whisper tiny English ONNX models](https://huggingface.co/onnx-community/whisper-tiny.en/tree/main/onnx)
- [Existing Groq batch transcription](https://console.groq.com/docs/speech-to-text)
