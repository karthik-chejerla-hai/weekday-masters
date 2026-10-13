# Grafana Cloud logs and metrics

Rally's API sends OTLP/HTTP protobuf directly to Grafana Cloud. No collector is
required for backend application telemetry. Export is off until `OTEL_ENABLED=true`.

## What is collected

- Existing Go `log` messages, including notification and scheduler messages.
- GORM query logs through the same bridge, with query parameters omitted.
- HTTP request logs with method, route template, status and duration.
- Request counts and duration histograms by method, route and response status.
- Go heap allocation and goroutine count.
- Database pool connections by used/idle state and cumulative connection waits.

Logs also stay on stderr for Cloud Logging. Legacy Go log messages have INFO severity.
HTTP logging omits raw URLs, query strings, headers and bodies; unmatched paths use
one `unmatched` label. Existing service messages can contain member UUIDs and error
details. Query parameter masking does not redact database error text.

This covers the API process, not browser console output, migration/seed commands,
Cloud Run platform logs/CPU metrics, Neon server metrics or distributed traces.
Those require separate integrations. SDK versions are pinned to support this
repository's Go 1.22 build and CI toolchain.

## 1. Get Grafana's connection settings

Open your Grafana Cloud stack's OpenTelemetry connection instructions and copy
the **OTLP endpoint** and generated **OTLP authorization headers**. Use a
stack-scoped access policy token with `logs:write` and `metrics:write`.

The endpoint is the base URL, typically ending in `/otlp`; the SDK appends
`/v1/logs` and `/v1/metrics`. Use HTTP/protobuf, not gRPC or Grafana's UI URL.
Keep the header secret. Use the generated header string exactly, including URL
encoding (for example `Authorization=Basic%20...`).

References: [Grafana OTLP connection instructions](https://grafana.com/docs/grafana-cloud/send-data/otlp/send-data-otlp/)
and [access policies](https://grafana.com/docs/grafana-cloud/platform/security-and-account-management/security-and-access/authentication-and-permissions/access-policies/).

## 2. Verify locally

Set these values in the ignored `backend/.env`:

```dotenv
OTEL_ENABLED=true
OTEL_SERVICE_NAME=rally-api
OTEL_RESOURCE_ATTRIBUTES=deployment.environment.name=local
OTEL_EXPORTER_OTLP_ENDPOINT=https://<your-otlp-host>/otlp
OTEL_EXPORTER_OTLP_HEADERS=<generated-header-string>
OTEL_EXPORTER_OTLP_TIMEOUT=3000
OTEL_METRIC_EXPORT_INTERVAL=30000
```

Start the backend normally against your local database. Request `/health` a few
times, then allow at least 30 seconds for metrics to export. Logs are batched.
SIGINT/SIGTERM drains HTTP requests and flushes exporters before exiting.

In Grafana Explore, choose the Logs data source and query:

```logql
{service_name="rally-api"} |= "HTTP"
```

In the Metrics data source, look for:

```promql
sum(rate(http_server_requests_total{service_name="rally-api"}[5m]))
```

```promql
histogram_quantile(0.95, sum by (le) (rate(http_server_request_duration_seconds_bucket{service_name="rally-api"}[5m])))
```

Grafana's metric translation settings can change suffixes; use the metric browser
to find `http_server_requests`, `http_server_request_duration`, `go_memory_heap`,
`go_goroutines`, `db_client_connections` and `db_client_connection_waits` if needed.
Each process has a unique `service.instance.id`, so concurrent Cloud Run instances
do not write to the same metric series.

## 3. Configure production

Create a Secret Manager secret named `rally-otel-headers` in your GCP project.
Store the complete generated `OTEL_EXPORTER_OTLP_HEADERS` value as its secret
payload, using Secret Manager's console so it stays out of shell history.

Grant **Secret Manager Secret Accessor** on that secret to the Cloud Run runtime
service account. This may differ from the GitHub deployment account. Find it with:

```bash
gcloud run services describe rally-club-api \
  --project <project-id> --region australia-southeast1 \
  --format='value(spec.template.spec.serviceAccountName)'
```

The deploying account must also be permitted to configure that secret reference.
Create these GitHub Actions variables in repository settings (or on the
`production-api` environment):

- `OTEL_EXPORTER_OTLP_ENDPOINT`: your Grafana Cloud base endpoint.
- `OTEL_HEADERS_SECRET`: `rally-otel-headers` (the secret name, never its payload).
- `OTEL_ENABLED`: `true` once the endpoint and secret are ready.

The production deploy workflow reads these variables, attaches the header as a
Secret Manager environment variable, and sets the production resource attribute.
It also uses `--no-cpu-throttling`: batching and periodic metric exports need CPU
outside request handling. **This selects instance-based Cloud Run billing** and
can change cost. It does not add minimum instances. See
[Cloud Run billing settings](https://docs.cloud.google.com/run/docs/configuring/billing-settings).

Deploy through the existing main-branch workflow after reviewing the change.
Preview revisions explicitly disable export so synthetic fixtures do not enter
production telemetry. The shared service means previews can still inherit its
CPU allocation setting. Verify production with `/health` and Explore as above.

## Troubleshooting and disabling

- Export failures remain on stderr as `OpenTelemetry export failed` and do not
  recursively generate OTLP logs. HTTP 401/403 usually means wrong headers,
  token realm or write permissions; 404 usually means the wrong base endpoint.
- An unavailable Grafana endpoint does not block HTTP requests; exporters batch,
  retry and use bounded queues. Telemetry can be dropped during long outages or
  forced termination. This is observability, not an audit archive.
- Set `OTEL_ENABLED=false` locally or in GitHub variables and redeploy to stop
  sending telemetry. This does not automatically restore request-based billing;
  restore CPU throttling explicitly if desired.
- Rotate the secret by adding a version and redeploying. Cloud Run reads the
  environment secret at instance startup; existing instances keep the old value.

## Tests

`cd backend && go test -race ./...` includes a local OTLP receiver test that checks
protobuf delivery, authentication, endpoint suffixes, request-data exclusion and
shutdown flushing. It needs no Grafana credentials or database.
