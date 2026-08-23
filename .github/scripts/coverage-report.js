// Aggregates the backend (go tool cover) and frontend (vitest json-summary)
// coverage artifacts into one PR comment plus a job summary.
//
// Kept out of the workflow YAML on purpose: the report is a JS template
// literal full of markdown tables, and inside a `script: |` block scalar any
// line that dedents to column 0 silently terminates the scalar and makes the
// whole workflow unparseable.
module.exports = async ({ github, context, core }) => {
  const fs = require('fs');

  let backendPct = 'N/A';
  let frontendPct = 'N/A';
  let feSummary = null;

  // 1. Parse backend coverage — the `total:` line of `go tool cover -func`.
  try {
    if (fs.existsSync('backend-coverage/coverage-summary.txt')) {
      const beFile = fs.readFileSync('backend-coverage/coverage-summary.txt', 'utf8');
      const totalLine = beFile.split('\n').find((l) => l.includes('total:'));
      if (totalLine) {
        const match = totalLine.match(/([0-9.]+)%/);
        if (match) backendPct = `${match[1]}%`;
      }
    }
  } catch (e) {
    core.warning(`Error reading backend coverage: ${e}`);
  }

  // 2. Parse frontend coverage.
  try {
    if (fs.existsSync('frontend-coverage/coverage-summary.json')) {
      feSummary = JSON.parse(fs.readFileSync('frontend-coverage/coverage-summary.json', 'utf8'));
      if (feSummary && feSummary.total && feSummary.total.lines) {
        frontendPct = `${feSummary.total.lines.pct}%`;
      }
    }
  } catch (e) {
    core.warning(`Error reading frontend coverage: ${e}`);
  }

  const getBadge = (pctStr) => {
    const num = parseFloat(pctStr);
    if (isNaN(num)) return '⚪ `N/A`';
    if (num >= 80) return `🟢 **${pctStr}**`;
    if (num >= 60) return `🟡 **${pctStr}**`;
    return `🔴 **${pctStr}**`;
  };

  // Job results arrive as env vars rather than `${{ }}` interpolation so the
  // script stays valid JS that can be linted and run outside Actions.
  const beJobPassed = process.env.BACKEND_RESULT === 'success';
  const feJobPassed = process.env.FRONTEND_RESULT === 'success';

  const detail = feSummary && feSummary.total
    ? [
        '<details>',
        '<summary>🔍 <b>Frontend Detailed Metrics</b></summary>',
        '',
        '| Metric | Percentage | Covered / Total |',
        '|---|:---:|:---:|',
        ...['lines', 'statements', 'branches', 'functions'].map((k) => {
          const m = feSummary.total[k];
          const label = k.charAt(0).toUpperCase() + k.slice(1);
          return `| **${label}** | ${m.pct}% | ${m.covered} / ${m.total} |`;
        }),
        '',
        '</details>',
      ].join('\n')
    : '';

  const markdown = [
    '## 📊 Test Coverage & CI Summary',
    '',
    '| Module | Line / Stmt Coverage | Tests Status |',
    '|---|:---:|:---:|',
    `| 🐹 **Backend (Go)** | ${getBadge(backendPct)} | ${beJobPassed ? '✅ Passing' : '❌ Failed'} |`,
    `| ⚛️ **Frontend (React)** | ${getBadge(frontendPct)} | ${feJobPassed ? '✅ Passing' : '❌ Failed'} |`,
    '',
    detail,
    '',
    `> 🕒 *Coverage computed for commit \`${context.sha.substring(0, 7)}\`.*`,
  ].join('\n');

  // 1. Output to the Actions step summary.
  await core.summary.addRaw(markdown).write();

  // 2. On a pull request, post or update a single sticky comment.
  if (context.payload.pull_request) {
    const prNumber = context.payload.pull_request.number;
    const marker = '<!-- rally-test-coverage-report -->';
    const body = `${marker}\n${markdown}`;

    try {
      const { data: comments } = await github.rest.issues.listComments({
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number: prNumber,
      });
      const existing = comments.find((c) => c.body && c.body.includes(marker));

      if (existing) {
        await github.rest.issues.updateComment({
          owner: context.repo.owner,
          repo: context.repo.repo,
          comment_id: existing.id,
          body,
        });
      } else {
        await github.rest.issues.createComment({
          owner: context.repo.owner,
          repo: context.repo.repo,
          issue_number: prNumber,
          body,
        });
      }
    } catch (err) {
      core.warning(`Failed to post/update sticky comment on PR: ${err}`);
    }
  }

  // The report job is what branch protection watches, so it must fail when
  // either suite failed.
  if (!beJobPassed || !feJobPassed) {
    core.setFailed('One or more test suites failed.');
  }
};
