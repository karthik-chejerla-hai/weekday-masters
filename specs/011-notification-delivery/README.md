# Notification implementation handoff

This branch stores the agreed notification specification, six-release plan and GPT 6 Luna handoff. Product decisions are confirmed; technical review prerequisites are recorded honestly and must be resolved per release.

## Resume on any machine

From a clone of `karthik-chejerla-hai/weekday-masters`:

```bash
git fetch origin
git switch codex/notification-delivery
```

If the local branch does not yet exist, use `git switch --track origin/codex/notification-delivery`. An isolated worktree may instead be created from that remote branch.

Give GPT 6 Luna [handoff-luna.md](handoff-luna.md). It directs implementation of **release 1 only**, tasks T001–T011, with validation. Read [review-followups.md](review-followups.md) before coding.

- [Product specification](spec.md)
- [Implementation plan](plan.md)
- [Release plan](release-plan.md)
- [HTML release plan](release-plan.html) — open locally in a browser
- [Task backlog](tasks.md)
- [Validation guide](quickstart.md)

For Spec Kit commands, set `SPECIFY_FEATURE_DIRECTORY=specs/011-notification-delivery`. The ignored local `.specify/feature.json` is not required when this environment variable is set. No credentials or private environment files are included.
