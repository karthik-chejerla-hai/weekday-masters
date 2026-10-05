# Validation, 5 October 2026

## Automated checks

- Full backend suite passed with `go test -race -coverprofile=coverage.out
  -covermode=atomic ./...` against the dedicated `rally_issue40_test` scratch DB.
  Go coverage after the attendance changes was 68.4%, above the CI floor of 60%.
- Additional race tests passed for session/member tool lookup and for the final
  actual-count history calculation. These verify Sydney date filtering, upcoming
  ordering, duplicate names, reduced provider data, and persisted actual counts.
- Frontend after the attendance changes: 236 tests passed in 37 files. Statement
  coverage was 75.91%, branch coverage 74.31%, function coverage 68.50%, and line
  coverage 77.84%.
- Frontend build passed. Vite reports the existing main-chunk size warning.
- ESLint passed with no errors. The existing `SessionDetail.tsx` effect-dependency
  warning remains. New files have no lint warnings.
- Nine Playwright tests passed against the real local frontend. Signed-out users
  cannot open assistant or expense routes.
- OpenAPI YAML parsed and all 269 internal references resolved. `bash -n` passed
  for the local runner. `git diff --check` passed.

The money tests cover exact equal shares, zero and invalid shuttle counts,
default confirmed RSVPs, member access, no preview writes, stale rates/stock/group,
duplicate confirmation, concurrent confirmations, two sessions competing for the
same stock, reversal, history snapshots, and the club balance identity. Provider
tests use a fake HTTP server. They check multipart audio, tool-call IDs/results,
safe quota/errors, missing configuration, cancellation, and model-specific options.
Browser recorder unit tests include cancellation, late permission, unmount, MIME
selection, and a late stop event from an earlier recording.

## Four requested attendance scenarios

`backend/internal/services/expense_scenarios_test.go` runs each scenario against
PostgreSQL with real confirmed RSVPs. Before each scenario, a previous two-hour
expense consumes 7 of 24 shuttles valued at $100.00. That leaves 17 shuttles worth
$70.83 and court credit of $440.00. A purchase adds 12 shuttles for $72.00, so the
next expense must use the blended 29-unit stock value of $142.83.

| Scenario | Count used | Court cost | Shuttle cost | Court credit after | Stock after |
| --- | ---: | ---: | ---: | ---: | --- |
| 2 hours, 6 confirmed | 8 | $60.00 | $39.40 | $380.00 | 21 units, $103.43 |
| 2 hours, 5 confirmed | 8 | $60.00 | $39.40 | $380.00 | 21 units, $103.43 |
| 2+1 hours, 5 confirmed | 10 | $83.00 | $49.25 | $357.00 | 19 units, $93.58 |
| 2+1 hours, 6 confirmed, one leaves after 2 | 10 | $83.00 | $49.25 | $357.00 | 19 units, $93.58 |

All four passed. In the last case, $32.83 of shuttle value belongs to the first
two hours and $16.42 to the extra hour. The early leaver pays $15.47 or $15.48,
depending on the deterministic remainder allocation. The other five share the
extra-hour court and shuttle costs. Every charge sums exactly to $132.25.

Each scenario checks preview and persisted assets, individual player balances,
the unchanged earlier expense and bank funds, saved history, duplicate refusal,
the ledger identity and reversal back to the post-purchase starting position.
Additional tests cover invalid extra-hour subsets, zero-to-five-cent time
allocation with an indivisible shuttle count, changed attendance after preview,
the HTTP route, assistant tool, form selection and history display.

The full Go race suite, frontend tests, lint and build passed after these changes.
The local app was restarted. Its API health check and frontend returned HTTP 200.

## Local checks

The app started with `scripts/local-issue40.sh`. The API health check returned
`{"status":"ok"}`. The Vite `/api/club` proxy returned the local seeded club.

The seed ran again without creating any members, sessions, RSVPs, transactions or
settlements. It preserved the owner's expense trial:

- Session: `d1633a1f-445d-46f6-b209-857f7cb71c4b`, 4 October 2026.
- Four confirmed RSVPs: Priya, Marcus, Aiko and Tom.
- No settlement for that session.
- Before the trial: 14 shuttles valued at 5833 cents, court credit 6000 cents.

Desktop (1440 px) and mobile (390 px) browser checks used the real React screens
with mocked authentication and API data in a temporary harness. They verified the
four default players, expense preview, separate confirmation, missing-key form
link, assistant preview, dashboard ordering, and member-only display. No page
errors or horizontal overflow occurred. Screenshots are in ignored
`tmp/issue40/{expense,assistant,home}-{desktop,mobile}.png`. The temporary harness
was removed after inspection. It never changed the demo database.

## Live Groq check after key setup

The owner added `GROQ_API_KEY` to the ignored backend environment file. The app
was restarted. Groq transcribed a 2.47-second synthetic WAV sample as “We played
three hours and used eight shuttles.” The actual assistant service then used the
live Groq planner and local database tools to prepare the seeded session expense:
3 hours, 8 shuttles, 4 players, and 11633 cents. The settlement count stayed zero.
The temporary read-only smoke program was removed after the check.

After adding early departure support, a second live Groq check passed against
synthetic data in the dedicated scratch database. The request specified three
hours, eight shuttles and Jordan Lee leaving after two hours. Groq selected six
players for the base band and five for the extra hour. Shuttle value split into
2222 and 1111 cents. Total cost was 11633 cents, with Jordan charged 1370 cents.
Assets and settlement count stayed unchanged. The temporary live test was removed;
normal automated tests do not need an API key. The six-player local demo session
was already settled, so this check did not use or change it.

The first smoke attempt used an empty audio file because macOS speech generation
did not work inside the sandbox. Generating the sample outside the sandbox
produced valid audio, and the same application call passed without code changes.

A real microphone and real Auth0 sign-in remain for the owner's browser trial.
The local app keeps the existing Google sign-in flow. Use the steps in
[quickstart.md](quickstart.md).

No production database, email delivery, Git commit, or push was part of this work.
