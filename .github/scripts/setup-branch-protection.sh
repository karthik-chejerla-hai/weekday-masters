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
# Only require checks from a workflow that runs on every pull request. A
# path-filtered workflow creates no check runs on a PR that misses its paths,
# and a required context that never reports leaves the PR pending forever
# rather than failing it. ci.yml has no paths filter for exactly this reason.
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
