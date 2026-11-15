# Tasks

- [x] Single role-aware introduction, compact composer and secondary expense link.
- [x] Role-aware example prompts populate focused editable input without sending.
- [x] Replace low-contrast amber microphone control with existing cyan outline colors.
- [x] All 528 frontend tests pass, including transcript/recording/review/save regressions; build and lint pass (one existing SessionDetail hook warning).
- [x] Orca browser desktop and 390px mobile: no overflow, editable examples focus message input, all composer buttons are at least 44px tall. Microphone text/icon rgb(21,94,117) on white measures 7.27:1 contrast (AA).

Eight targeted desktop/mobile composer and game browser scenarios pass, including editable examples, provider errors retaining text, member/admin actions and explicit score saves. Browser test TypeScript check passes. The initial new browser fixture used the wrong route; corrected it to the existing `/assistant/messages` endpoint and reran all eight scenarios successfully.
