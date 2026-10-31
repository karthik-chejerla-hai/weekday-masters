// Notify current collaborators only. Never use commit author addresses or expose
// recipient addresses, provider responses, credentials, or build logs in output.
function validEmail(value) {
  return typeof value === 'string' && /^[^\s@<>]+@[^\s@<>]+\.[^\s@<>]+$/.test(value)
    && !value.toLowerCase().endsWith('@users.noreply.github.com')
}

module.exports = async function notify({ github, context, core, env = process.env, fetchImpl = fetch }) {
  const run = context.payload.workflow_run
  if (!run || run.conclusion !== 'failure' || run.head_branch !== 'main'
      || run.head_repository?.full_name !== `${context.repo.owner}/${context.repo.repo}`
      || !['push', 'workflow_dispatch'].includes(run.event)) return

  if (!env.SENDGRID_API_KEY || !validEmail(env.SENDGRID_FROM_EMAIL)) {
    throw new Error('Configure SENDGRID_API_KEY and SENDGRID_FROM_EMAIL for failure alerts.')
  }
  let overrides
  try {
    overrides = JSON.parse(env.CI_COLLABORATOR_EMAILS || '{}')
    if (!overrides || Array.isArray(overrides) || typeof overrides !== 'object') throw new Error()
  } catch {
    throw new Error('CI_COLLABORATOR_EMAILS must be a JSON object mapping GitHub logins to email addresses.')
  }
  const collaborators = await github.paginate(github.rest.repos.listCollaborators, {
    ...context.repo, affiliation: 'all', per_page: 100,
  })
  const recipients = new Map()
  const missing = []
  for (const collaborator of collaborators) {
    // GitHub App bots do not have human mailboxes.
    if (collaborator.type === 'Bot') continue
    let email = overrides[collaborator.login]
    if (!Object.hasOwn(overrides, collaborator.login)) {
      const { data } = await github.rest.users.getByUsername({ username: collaborator.login })
      email = data.email
    }
    if (!validEmail(email)) {
      missing.push(collaborator.login)
      continue
    }
    recipients.set(email.toLowerCase(), email)
  }

  if (recipients.size === 0) {
    throw new Error('No deliverable collaborator addresses. Configure CI_COLLABORATOR_EMAILS.')
  }
  const jobs = await github.paginate(github.rest.actions.listJobsForWorkflowRun, {
    ...context.repo, run_id: run.id, filter: 'latest', per_page: 100,
  })
  const failed = jobs.filter(job => job.conclusion === 'failure')
  const details = failed.length
    ? failed.map(job => `- ${job.name}: ${job.html_url}`).join('\n')
    : 'Open the run link for details. The workflow may have failed before a job started.'
  const text = [
    `The main pipeline failed for ${context.repo.owner}/${context.repo.repo}.`,
    `Commit: ${run.head_sha}`,
    `Run: ${run.html_url}`,
    `Attempt: ${run.run_attempt}`,
    '', 'Failed jobs:', details,
    '', 'Production may still use the previous version or may be partially updated. Check the failed jobs before retrying.',
  ].join('\n')
  const addresses = [...recipients.values()]
  // SendGrid allows at most 1,000 personalizations per request. Separate
  // envelopes keep collaborator email addresses private from one another.
  for (let offset = 0; offset < addresses.length; offset += 1000) {
    const response = await fetchImpl('https://api.sendgrid.com/v3/mail/send', {
      method: 'POST',
      headers: { Authorization: `Bearer ${env.SENDGRID_API_KEY}`, 'Content-Type': 'application/json' },
      signal: AbortSignal.timeout(30000),
      body: JSON.stringify({
        from: { email: env.SENDGRID_FROM_EMAIL, name: 'Rally CI' },
        personalizations: addresses.slice(offset, offset + 1000).map(email => ({ to: [{ email }] })),
        subject: `[${context.repo.repo}] main pipeline failed (${run.head_sha.slice(0, 7)})`,
        content: [{ type: 'text/plain', value: text }],
      }),
    })
    // Do not retry automatically: a timeout can occur after SendGrid accepted
    // the request. Retrying could send duplicate messages.
    if (response.status !== 202) throw new Error(`SendGrid rejected the failure alert (HTTP ${response.status}).`)
  }
  core.info(`SendGrid accepted failure alerts for ${addresses.length} distinct collaborator addresses.`)
  if (missing.length) {
    core.setFailed(`Missing deliverable email addresses for: ${missing.join(', ')}. Update CI_COLLABORATOR_EMAILS.`)
  }
}
