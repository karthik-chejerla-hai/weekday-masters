# Research

## Decision: Groq with two small interfaces

Use Whisper Large V3 Turbo for speech and GPT-OSS 120B for tool requests. net/http
avoids an SDK dependency. Separate Transcriber and Planner interfaces permit
independent replacement. Set parallel_tool_calls=false, include_reasoning=false,
and reasoning_effort=low. Never expose a money-writing tool to the model.

Sources: https://console.groq.com/docs/speech-to-text,
https://console.groq.com/docs/tool-use/local-tool-calling,
https://console.groq.com/docs/tool-use/overview,
https://console.groq.com/docs/reasoning.

## Decision: browser recording with typed fallback

Probe MediaRecorder MIME support and use its actual output type. Limit recordings
to 60 seconds and uploads to 10 MB. Stop tracks on completion, cancellation and
unmount. Microphone access needs HTTPS or localhost. Display the transcript for
review before sending. A form remains usable without any model provider.

Sources: https://developer.mozilla.org/en-US/docs/Web/API/MediaRecorder/isTypeSupported_static,
https://developer.mozilla.org/en-US/docs/Web/API/MediaDevices/getUserMedia.

## Decision: reuse settlement and ledger

ActualShuttles is nullable on settlements. Nil retains historical estimated band
semantics. Actual-count inputs consume stock once. For three hours, shuttle value
is allocated 2:1 by time using largest-remainder rounding. Each band's court and
shuttle costs are shared by its own participants. When everyone stays, split the
entire cost in one allocation to keep differences within one cent.
A deterministic preview fingerprint covers canonical inputs, rates and costed
output. Confirmation recalculates under account locks and rejects changes.
The assistant only prepares previews. Existing advanced settlement routes stay
available. No separate AI database writer or new agent framework is needed.

## Decision: isolated local data

The existing backend .env targets Neon. The local runner must explicitly override
DATABASE_URL to loopback and disable notifications. Use the existing synthetic
seed, which has an unsettled past session with four RSVPs. Do not modify or copy
production data. Existing Auth0 configuration supplies local Google sign-in.
