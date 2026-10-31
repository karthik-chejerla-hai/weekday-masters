const { test } = require('node:test')
const assert = require('node:assert/strict')
const notify = require('./ci-failure-email')

function fixture() {
  const requests = []
  const errors = []
  const profiles = []
  const collaborators = [
    { login: 'private-user', type: 'User' },
    { login: 'public-user', type: 'User' },
    { login: 'app[bot]', type: 'Bot' },
  ]
  const jobs = [{ name: 'Deploy Backend', conclusion: 'failure', html_url: 'https://github.com/owner/repo/actions/runs/42/job/1' }]
  const listCollaborators = () => {}
  const listJobsForWorkflowRun = () => {}
  const args = {
    github: {
      rest: {
        repos: { listCollaborators },
        actions: { listJobsForWorkflowRun },
        users: { getByUsername: async ({ username }) => {
          profiles.push(username)
          return { data: { email: username === 'public-user' ? 'public@example.com' : null } }
        } },
      },
      paginate: async (endpoint, options) => {
        assert.equal(options.per_page, 100)
        if (endpoint === listCollaborators) {
          assert.equal(options.affiliation, 'all')
          return collaborators
        }
        assert.equal(endpoint, listJobsForWorkflowRun)
        assert.equal(options.filter, 'latest')
        assert.equal(options.run_id, 42)
        return jobs
      },
    },
    context: {
      repo: { owner: 'owner', repo: 'repo' },
      payload: { workflow_run: {
        id: 42, conclusion: 'failure', head_branch: 'main', event: 'push',
        head_repository: { full_name: 'owner/repo' }, head_sha: 'a'.repeat(40),
        html_url: 'https://github.com/owner/repo/actions/runs/42', run_attempt: 2,
      } },
    },
    core: { info: () => {}, setFailed: message => errors.push(message) },
    env: {
      SENDGRID_API_KEY: 'test-key', SENDGRID_FROM_EMAIL: 'ci@example.com',
      CI_COLLABORATOR_EMAILS: JSON.stringify({ 'private-user': 'private@example.com', 'removed-user': 'removed@example.com' }),
    },
    fetchImpl: async (url, options) => { requests.push({ url, options }); return { status: 202 } },
  }
  return { args, requests, errors, profiles, collaborators, jobs }
}

test('emails all current human collaborators using private overrides and public profiles', async () => {
  const f = fixture()
  await notify(f.args)
  assert.equal(f.requests.length, 1)
  assert.equal(f.requests[0].url, 'https://api.sendgrid.com/v3/mail/send')
  const body = JSON.parse(f.requests[0].options.body)
  assert.deepEqual(body.personalizations, [
    { to: [{ email: 'private@example.com' }] }, { to: [{ email: 'public@example.com' }] },
  ])
  assert.deepEqual(f.profiles, ['public-user'])
  assert.match(body.content[0].value, /Deploy Backend/)
  assert.match(body.content[0].value, /Attempt: 2/)
  assert.doesNotMatch(f.requests[0].options.body, /removed@example.com|test-key/)
  assert.deepEqual(f.errors, [])
})

test('does not email for successful, cancelled, PR, other-branch, or foreign-repository runs', async () => {
  for (const change of [{ conclusion: 'success' }, { conclusion: 'cancelled' }, { event: 'pull_request' },
    { head_branch: 'feature' }, { head_repository: { full_name: 'fork/repo' } }]) {
    const f = fixture()
    Object.assign(f.args.context.payload.workflow_run, change)
    f.args.github.paginate = () => { throw new Error('GitHub must not be called') }
    await notify(f.args)
    assert.equal(f.requests.length, 0)
  }
})

test('manual main failures and failures before job creation still send a run link', async () => {
  const f = fixture()
  f.args.context.payload.workflow_run.event = 'workflow_dispatch'
  f.jobs.length = 0
  await notify(f.args)
  assert.match(JSON.parse(f.requests[0].options.body).content[0].value, /may have failed before a job started/)
})

test('sends to reachable users but fails visibly when a collaborator address is missing', async () => {
  const f = fixture()
  f.args.env.CI_COLLABORATOR_EMAILS = '{}'
  await notify(f.args)
  assert.equal(f.requests.length, 1)
  assert.match(f.errors[0], /private-user/)
  assert.equal(JSON.parse(f.requests[0].options.body).personalizations.length, 1)
})

test('deduplicates addresses across accounts', async () => {
  const f = fixture()
  f.args.env.CI_COLLABORATOR_EMAILS = '{"private-user":"PUBLIC@example.com"}'
  await notify(f.args)
  assert.equal(JSON.parse(f.requests[0].options.body).personalizations.length, 1)
})

test('rejects invalid configuration without sending mail', async () => {
  for (const change of [{ SENDGRID_API_KEY: '' }, { SENDGRID_FROM_EMAIL: 'bad' },
    { CI_COLLABORATOR_EMAILS: 'not json' }, { CI_COLLABORATOR_EMAILS: '[]' }]) {
    const f = fixture()
    Object.assign(f.args.env, change)
    await assert.rejects(notify(f.args), /Configure|must be a JSON object/)
    assert.equal(f.requests.length, 0)
  }
})

test('does not use noreply addresses or send to removed collaborators', async () => {
  const f = fixture()
  f.collaborators.splice(1)
  f.args.env.CI_COLLABORATOR_EMAILS = '{"private-user":"1+user@users.noreply.github.com","removed-user":"removed@example.com"}'
  await assert.rejects(notify(f.args), /No deliverable/)
  assert.equal(f.requests.length, 0)
})

test('provider failures fail the job without exposing response data or retrying', async () => {
  const f = fixture()
  let calls = 0
  f.args.fetchImpl = async () => { calls++; return { status: 403, text: () => 'secret' } }
  await assert.rejects(notify(f.args), { message: 'SendGrid rejected the failure alert (HTTP 403).' })
  assert.equal(calls, 1)
})

test('splits large collaborator lists into private SendGrid envelopes', async () => {
  const f = fixture()
  f.collaborators.length = 0
  const overrides = {}
  for (let i = 0; i < 1001; i++) {
    f.collaborators.push({ login: `user-${i}`, type: 'User' })
    overrides[`user-${i}`] = `user-${i}@example.com`
  }
  f.args.env.CI_COLLABORATOR_EMAILS = JSON.stringify(overrides)
  await notify(f.args)
  assert.deepEqual(f.requests.map(r => JSON.parse(r.options.body).personalizations.length), [1000, 1])
})
