// GitHub-hosted badges, derived only from successful main-branch CI reports.
const fs = require('node:fs');

function readCoverage(directory = '.') {
  const backend = fs.readFileSync(`${directory}/backend-coverage/coverage-summary.txt`, 'utf8');
  const frontend = JSON.parse(fs.readFileSync(`${directory}/frontend-coverage/coverage-summary.json`, 'utf8'));
  const match = backend.match(/^total:\s+\(statements\)\s+([\d.]+)%\s*$/m);
  const metrics = { backend: match ? Number(match[1]) : NaN, frontend: frontend.total?.lines?.pct };
  for (const [name, value] of Object.entries(metrics)) {
    if (typeof value !== 'number' || !Number.isFinite(value) || value < 0 || value > 100) {
      throw new Error(`Missing or invalid ${name} coverage`);
    }
  }
  return metrics;
}

function badge(label, percentage) {
  const value = `${percentage.toFixed(1)}%`;
  const color = percentage >= 80 ? '#4c1' : percentage >= 60 ? '#dfb317' : '#e05d44';
  return `<svg xmlns="http://www.w3.org/2000/svg" width="220" height="20" role="img" aria-label="${label}: ${value}">
  <title>${label}: ${value}</title>
  <clipPath id="clip"><rect width="220" height="20" rx="3"/></clipPath>
  <g clip-path="url(#clip)"><path fill="#555" d="M0 0h166v20H0z"/><path fill="${color}" d="M166 0h54v20h-54z"/></g>
  <g fill="#fff" text-anchor="middle" font-family="Verdana,DejaVu Sans,sans-serif" font-size="11">
    <text x="83" y="14">${label}</text><text x="193" y="14">${value}</text>
  </g>
</svg>\n`;
}

async function publish({ github, context, core }, metrics) {
  if (context.eventName !== 'push' || context.ref !== 'refs/heads/main') {
    throw new Error('Coverage badges can only be published by a main push');
  }
  const repo = context.repo;
  const { data: main } = await github.rest.git.getRef({ ...repo, ref: 'heads/main' });
  if (main.object.sha !== context.sha) {
    core.info('Skipping an older run because main has advanced.');
    return;
  }
  const ref = 'heads/coverage-badges';
  let parent;
  let baseTree;
  try {
    const { data } = await github.rest.git.getRef({ ...repo, ref });
    parent = data.object.sha;
  } catch (error) {
    if (error.status !== 404) throw error;
  }
  if (parent) {
    const { data } = await github.rest.git.getCommit({ ...repo, commit_sha: parent });
    baseTree = data.tree.sha;
  }
  const report = {
    commit: context.sha,
    run: `https://github.com/${repo.owner}/${repo.repo}/actions/runs/${context.runId}`,
    backend_statements_percent: metrics.backend,
    frontend_lines_percent: metrics.frontend,
  };
  const files = {
    'backend.svg': badge('backend statements', metrics.backend),
    'frontend.svg': badge('frontend lines', metrics.frontend),
    'coverage.json': JSON.stringify(report, null, 2) + '\n',
  };
  const { data: tree } = await github.rest.git.createTree({ ...repo,
    ...(baseTree ? { base_tree: baseTree } : {}),
    tree: Object.entries(files).map(([path, content]) => ({ path, mode: '100644', type: 'blob', content })),
  });
  const { data: commit } = await github.rest.git.createCommit({ ...repo,
    message: `Update coverage for ${context.sha}`, tree: tree.sha, parents: parent ? [parent] : [],
  });
  if (parent) {
    await github.rest.git.updateRef({ ...repo, ref, sha: commit.sha, force: false });
  } else {
    await github.rest.git.createRef({ ...repo, ref: `refs/${ref}`, sha: commit.sha });
  }
  core.info(`Published coverage badges for ${context.sha}`);
}

module.exports = { readCoverage, badge, publish };
