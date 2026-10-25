# Production logs, metrics and billing

Production and pull request previews use Google Cloud's built-in Cloud Run
request/platform logs and metrics. Rally writes application logs to stdout/stderr,
where Cloud Logging collects them. The Go OTLP instrumentation and authenticated
browser-error instrumentation remain in the codebase for local use, but both
deployment workflows set `OTEL_ENABLED=false` and remove OTLP endpoint/header
bindings. Browser clients send only allowlisted error category/type events to the
API; with the exporter disabled, the API does not forward them to Grafana. No
Grafana OTLP logs or metrics are exported by production or preview revisions.

Cloud Run deployments explicitly use request-based billing (`--cpu-throttling`)
with zero service- and revision-level minimum instances. This allows scale to
zero. Preview revisions have no default traffic, but their tagged URLs remain
callable: requests to a preview can start instances and incur normal request-based
charges. The preview workflow applies the same billing and telemetry settings
because previews and production share one Cloud Run service.

## Local Grafana verification

To exercise the retained exporter locally, set the following in the ignored
`backend/.env` and use a test Grafana stack or local OTLP receiver. Never copy
these values to production or GitHub Actions.

```dotenv
OTEL_ENABLED=true
OTEL_SERVICE_NAME=rally-api
OTEL_RESOURCE_ATTRIBUTES=deployment.environment.name=local
OTEL_EXPORTER_OTLP_ENDPOINT=https://<your-otlp-host>/otlp
OTEL_EXPORTER_OTLP_HEADERS=<generated-header-string>
OTEL_EXPORTER_OTLP_TIMEOUT=3000
OTEL_METRIC_EXPORT_INTERVAL=30000
```

The endpoint is the base URL, typically ending in `/otlp`; the SDK appends
`/v1/logs` and `/v1/metrics`. Use HTTP/protobuf, not gRPC or Grafana's UI URL.
Keep the header secret and use the generated header string exactly, including
URL encoding. See [Grafana OTLP connection
instructions](https://grafana.com/docs/grafana-cloud/send-data/otlp/send-data-otlp/)
and [access policies](https://grafana.com/docs/grafana-cloud/platform/security-and-account-management/security-and-access/authentication-and-permissions/access-policies/).

Start the backend against a local database, request `/health`, and allow at least
30 seconds for metrics export. In Grafana Explore, the Logs data source query
`{service_name="rally-api"} |= "HTTP"` shows request logs. Browser error
categories use `{service_name="rally-api"} |= "Browser client error"`. Metrics
include `http_server_requests`, `http_server_request_duration`, `go_memory_heap`,
`go_goroutines`, `db_client_connections` and `db_client_connection_waits`.
SIGINT/SIGTERM drains requests and flushes exporters. Local receiver tests cover
protobuf delivery, authentication, endpoint suffixes, request-data exclusion and
shutdown flushing without Grafana credentials or a database.

## Production operational views

- [Logs Explorer](https://console.cloud.google.com/logs/query): select resource
  type `cloud_run_revision` and filter to service `rally-club-api` in
  `australia-southeast1`.
- [Cloud Run service metrics](https://console.cloud.google.com/run/detail/australia-southeast1/rally-club-api/metrics): inspect request count and latency, instance count, CPU and memory.
- [Cloud Monitoring Metrics Explorer](https://console.cloud.google.com/monitoring/metrics-explorer): use resource type **Cloud Run Revision** and metrics such as request count, request latencies, billable instance time, container CPU utilization, memory utilization and instance count.
- [Cloud Run service revisions](https://console.cloud.google.com/run/detail/australia-southeast1/rally-club-api/revisions): verify the serving revision, traffic allocation, revision settings and tagged previews.

Useful Logs Explorer queries (adjust the project and time range in the console):

```text
resource.type="cloud_run_revision"
resource.labels.service_name="rally-club-api"
resource.labels.location="australia-southeast1"
```

```text
resource.type="cloud_run_revision"
resource.labels.service_name="rally-club-api"
resource.labels.location="australia-southeast1"
severity>=ERROR
```

```text
resource.type="cloud_run_revision"
resource.labels.service_name="rally-club-api"
resource.labels.location="australia-southeast1"
(logName="projects/PROJECT_ID/logs/run.googleapis.com%2Fstdout" OR
 logName="projects/PROJECT_ID/logs/run.googleapis.com%2Fstderr")
```

To inspect request logs, filter
`logName="projects/PROJECT_ID/logs/run.googleapis.com%2Frequests"`. Request logs
include status, latency and route URL. Application messages can contain member
UUIDs and error details; operational logs are not an audit archive. Avoid broad
exports and extended retention unless there is a specific need.

## Cost notes

Request-based Cloud Run billing charges for request processing and startup and
shutdown, not idle CPU between requests. At zero minimum instances an idle service
can scale down, with possible cold starts. The Cloud Run free tier currently
includes, across projects on a billing account, 2 million requests, 180,000
vCPU-seconds and 360,000 GiB-seconds per month for request-based services. Usage
beyond the free tier, outbound transfer and other Google Cloud resources can still
cost money; the free tier is not a project-level budget cap.

Cloud Logging currently includes 50 GiB of eligible log storage per project per
month. Additional eligible ingestion and retention beyond the default period may
be charged. Cloud Run's built-in metrics have no charge for fully managed Cloud
Run, but other Monitoring features, alerts, uptime checks, log-based metrics and
retention may have separate pricing. Check current scope and rates before changing
retention or adding features.

- [Cloud Run pricing and free tier](https://cloud.google.com/run/pricing)
- [Cloud Logging pricing and free allotment](https://cloud.google.com/logging/pricing)
- [Cloud Run monitoring and built-in metrics](https://docs.cloud.google.com/run/docs/monitoring)
- [Google Cloud Observability pricing](https://cloud.google.com/stackdriver/pricing)

Custom Grafana charts and alerts for Go runtime, HTTP middleware and database
pool metrics stop receiving new data. Historical Grafana data and the Grafana
account remain intact. Built-in Cloud Run metrics do not replace those custom
application/database-pool measurements.

## Rollout and verification

The production and preview workflows set request-based billing, both minimum
instance settings to zero, and `OTEL_ENABLED=false`; they remove old OTLP endpoint
and header bindings when creating a revision. After the updated production
workflow has deployed successfully, remove the unused `OTEL_ENABLED`,
`OTEL_EXPORTER_OTLP_ENDPOINT` and `OTEL_HEADERS_SECRET` GitHub Actions variables.
The workflow does not delete the Secret Manager secret, Grafana account or
historical Grafana data.

For production, save a redacted pre-change service/revision configuration and
verify after deployment that the serving revision is ready, `OTEL_ENABLED` is
false, CPU throttling is enabled and service/revision minimum instances are zero.
Check `/health`, authenticated app reads and writes, application stdout/stderr,
Cloud Run request logs and built-in metrics. Use synthetic checks only; do not
send real member notifications for verification.

Review each retained tagged preview revision separately. Zero default traffic
does not mean a tagged URL cannot run or incur charges. After this change lands on
the base branch, trigger a fresh `synchronize` run for each active PR (a no-op
commit may be needed); rerunning an older workflow run can use its old definition.
Verify each existing `pr-N` tag points to the new ready revision with
request-based billing and OTLP disabled, and that the old revision no longer has
the tag. Review closed PRs against the cleanup workflow. Coordinate retirement
with owners and keep active previews rather than deleting tags indiscriminately.

Save post-change service/revision settings with secrets redacted and compare daily
gross and net Cloud Run costs after billing data settles. Do not attribute every
prior project charge to telemetry or promise zero cost.

Request-based billing suspends background CPU while no request is being handled.
The hourly reminder scheduler may therefore run late or unreliably during idle
periods; this is an accepted interim tradeoff tracked in issue #99. User-triggered
push notifications and explicit invitation delivery continue under their
existing preferences and controls.

## Rollback

The exporter code and dependencies remain available for local diagnosis. Restoring
production export requires a reviewed workflow change, an endpoint and existing
Secret Manager reference, and explicit acceptance of instance-based billing if
always-allocated CPU is needed. Do not re-enable export by changing only a runtime
variable or delete historical data, unrelated credentials, the Grafana account
or the existing secret as part of rollback.
