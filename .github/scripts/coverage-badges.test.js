const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { readCoverage, badge, publish } = require('./coverage-badges');

test('reads actual Go totals and frontend line coverage, rejects missing results', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'rally-coverage-'));
  try {
    fs.mkdirSync(`${dir}/backend-coverage`);
    fs.mkdirSync(`${dir}/frontend-coverage`);
    fs.writeFileSync(`${dir}/backend-coverage/coverage-summary.txt`, 'file.go:1: f 100.0%\ntotal:\t(statements)\t62.3%\n');
    fs.writeFileSync(`${dir}/frontend-coverage/coverage-summary.json`, JSON.stringify({ total: { lines: { pct: 80.33 }, statements: { pct: 78.28 } } }));
    assert.deepEqual(readCoverage(dir), { backend: 62.3, frontend: 80.33 });
    fs.writeFileSync(`${dir}/backend-coverage/coverage-summary.txt`, 'file.go:1: f 100.0%\n');
    assert.throws(() => readCoverage(dir), /invalid backend/);
    fs.writeFileSync(`${dir}/backend-coverage/coverage-summary.txt`, 'total: (statements) 62.3%\n');
    fs.writeFileSync(`${dir}/frontend-coverage/coverage-summary.json`, '{"total":{"lines":{"pct":null}}}');
    assert.throws(() => readCoverage(dir), /invalid frontend/);
  } finally { fs.rmSync(dir, { recursive: true }); }
});

test('badge labels distinguish the metrics and round only the display', () => {
  assert.match(badge('frontend lines', 80.33), /frontend lines: 80.3%/);
  assert.match(badge('backend statements', 59.9), /fill="#e05d44"/);
});

function harness({ stale = false, existing = false } = {}) {
  const writes = [];
  const git = {
    getRef: async ({ ref }) => {
      if (ref === 'heads/main') return { data: { object: { sha: stale ? 'newer' : 'tested' } } };
      if (existing) return { data: { object: { sha: 'parent' } } };
      throw Object.assign(new Error('Not found'), { status: 404 });
    },
    getCommit: async () => ({ data: { tree: { sha: 'old-tree' } } }),
    createTree: async args => { writes.push(['tree', args]); return { data: { sha: 'tree' } }; },
    createCommit: async args => { writes.push(['commit', args]); return { data: { sha: 'commit' } }; },
    createRef: async args => writes.push(['create', args]),
    updateRef: async args => writes.push(['update', args]),
  };
  return { writes, input: { github: { rest: { git } }, core: { info() {} },
    context: { repo: { owner: 'test', repo: 'test' }, eventName: 'push', ref: 'refs/heads/main', sha: 'tested', runId: 42 } } };
}
const metrics = { backend: 62.3, frontend: 80.33 };
test('first publication creates a branch containing only badges and source metadata', async () => {
  const { input, writes } = harness();
  await publish(input, metrics);
  assert.deepEqual(writes[0][1].tree.map(file => file.path), ['backend.svg', 'frontend.svg', 'coverage.json']);
  assert.deepEqual(writes[1][1].parents, []);
  assert.equal(writes[2][1].ref, 'refs/heads/coverage-badges');
  assert.equal(JSON.parse(writes[0][1].tree[2].content).commit, 'tested');
});
test('updates preserve badge branch history without force', async () => {
  const { input, writes } = harness({ existing: true });
  await publish(input, metrics);
  assert.equal(writes[0][1].base_tree, 'old-tree');
  assert.deepEqual(writes[1][1].parents, ['parent']);
  assert.equal(writes[2][0], 'update');
  assert.equal(writes[2][1].force, false);
});
test('an outdated main run cannot publish', async () => {
  const { input, writes } = harness({ stale: true });
  await publish(input, metrics);
  assert.deepEqual(writes, []);
});
test('a pull request cannot publish', async () => {
  const { input, writes } = harness();
  input.context.eventName = 'pull_request';
  await assert.rejects(publish(input, metrics), /main push/);
  assert.deepEqual(writes, []);
});
