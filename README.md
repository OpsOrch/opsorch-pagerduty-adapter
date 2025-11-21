# OpsOrch PagerDuty Adapter

This module integrates OpsOrch with PagerDuty using the PagerDuty REST API v2. It implements the incident provider interface to create, query, retrieve, and update PagerDuty incidents directly from OpsOrch workflows.

## Layout
- `incident/pagerduty_provider.go`: incident provider implementation plus config helpers and registry wiring.
- `cmd/incidentplugin/main.go`: JSON-RPC plugin entrypoint so the adapter can run out-of-process.
- `version.go`: adapter version + minimum OpsOrch Core requirement.
- `Makefile`: build/test/plugin shortcuts with a local module cache.

## Development
```bash
make test      # runs go test ./...
make build     # go build ./...
make plugin    # builds ./bin/incidentplugin
```

`go.mod` points at the sibling `opsorch-core` repo for local iteration. Remove the replace directive when publishing.

## Configuration Contract

```json
{
  "source": "pagerduty",
  "defaultSeverity": "critical",
  "apiToken": "your_pagerduty_api_token",
  "apiURL": "https://api.pagerduty.com",
  "serviceID": "PXXXXXX",
  "fromEmail": "user@example.com"
}
```

- `apiToken` is required; this is your PagerDuty API token (REST API key).
- `apiURL` defaults to "https://api.pagerduty.com" but can point at mocks during development.
- `serviceID` is required; this is the PagerDuty service ID where incidents will be created.
- `fromEmail` is required; this must be a valid email address of a PagerDuty user in your account.
- `defaultSeverity` defaults to "critical" and maps to PagerDuty urgency levels.

## Using In OpsOrch Core
Import the module for side effects and select it via `OPSORCH_INCIDENT_PROVIDER=pagerduty`, or build the plugin and set `OPSORCH_INCIDENT_PLUGIN` to the binary path.

## Plugin RPC Contract
OpsOrch Core talks to the plugin over stdin/stdout using JSON objects shaped like:

```json
{
  "method": "incident.create",
  "config": { /* decrypted OPSORCH_INCIDENT_CONFIG */ },
  "payload": { /* method-specific body */ }
}
```

- `config` is the decrypted map described above; Core injects it on every call so the plugin never stores secrets on disk.
- `payload` matches the schema from `opsorch-core` for the requested method (e.g., `schema.CreateIncidentInput` for `incident.create`).
- Responses mirror `{ "result": any }` or `{ "error": string }` for success/failure.

Because the protocol stays on-box (pipes between Core and the plugin), PagerDuty credentials remain local. Avoid logging the config or token in the plugin, and rotate the `apiToken` at the cadence required by your organization.


## PagerDuty API Integration

This adapter integrates with the PagerDuty REST API v2:

- **Create** → POST `/incidents` - Creates new PagerDuty incidents
- **Get** → GET `/incidents/{id}` - Retrieves incident details
- **List** → GET `/incidents` - Lists incidents for the configured service
- **Query** → GET `/incidents` - Searches incidents with filters
- **Update** → PUT `/incidents/{id}` - Updates incident fields and status
- **GetTimeline** → GET `/incidents/{id}/log_entries` - Retrieves incident log entries
- **AppendTimeline** → POST `/incidents/{id}/notes` - Adds notes to incidents

### Authentication

The adapter uses Token-based authentication with PagerDuty API tokens. Generate an API token from your PagerDuty account:
1. Go to Configuration → API Access
2. Create a new API Key (REST API v2)
3. Use the token in the `apiToken` configuration field

### Severity and Status Mapping

**Severity to Urgency:**
- `critical`, `sev1`, `p1` → `high`
- `high`, `sev2`, `p2` → `high`
- `medium`, `sev3`, `p3` → `low`
- `low`, `sev4`, `p4` → `low`

**Status Mapping:**
- `open`, `triggered` → `triggered`
- `acknowledged`, `investigating` → `acknowledged`
- `resolved`, `closed` → `resolved`

### Service ID

To find your PagerDuty service ID:
1. Go to Services → Service Directory
2. Click on your service
3. The service ID is in the URL: `https://[your-subdomain].pagerduty.com/services/PXXXXXX`
