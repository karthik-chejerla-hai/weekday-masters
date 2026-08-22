#!/usr/bin/env bash
# Protects main and turns on auto-merge.
#
# Run this as a repository ADMIN — branch protection is admin-only, and a
# collaborator with push access gets 403. Check with:
#
#   gh api repos/OWNER/REPO --jq .permissions
#
# Usage:
#   .github/scripts/setup-branch-protection.sh [owner/repo]
set -euo pipefail

repo="${1:-$(gh repo view --json nameWithOwner --jq .nameWithOwner)}"
branch="${BRANCH:-main}"

echo "Protecting ${repo}@${branch}..."

# The three jobs of the CI & Test Coverage workflow.
#
# Never add "build" here, whether or not ci-backend.yml and ci-frontend.yml
# still exist. Both name their job "build", so the context is ambiguous between
# them — and both filter on paths, so on a PR that touches neither backend/ nor
# frontend/ no check run by that name is ever created. A required context that
# never reports leaves the PR pending forever rather than failing it.
#
# Those two workflows are a subset of this one and can be deleted whenever;
# nothing here depends on that.
gh api -X PUT "repos/${repo}/branches/${branch}/protection" \
  --input - <<'JSON'
{
  "required_status_checks": {
    "strict": false,
    "contexts": [
      "Backend Tests & Coverage",
      "Frontend Tests & Coverage",
      "Aggregate Coverage & Decorate PR"
    ]
  },
  "enforce_admins": false,
  "required_pull_request_reviews": null,
  "restrictions": null,
  "required_linear_history": true,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "required_conversation_resolution": true
}
JSON

# "strict" is off on purpose: with a stack of PRs, requiring every branch to be
# up to date with main forces a rebase of the whole stack every time its bottom
# lands. Correctness comes from the checks, which re-run on each push.

echo "Enabling auto-merge and tidying merged branches..."
gh api -X PATCH "repos/${repo}" \
  -F allow_auto_merge=true \
  -F delete_branch_on_merge=true \
  -F allow_squash_merge=true \
  --silent

echo
echo "Done. Current protection:"
gh api "repos/${repo}/branches/${branch}/protection" \
  --jq '{checks: .required_status_checks.contexts, strict: .required_status_checks.strict, linear: .required_linear_history.enabled}'

echo
echo "Auto-merge a PR once its checks pass with:"
echo "  gh pr merge <number> --squash --auto"
