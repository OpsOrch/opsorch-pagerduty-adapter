# OpsOrch PagerDuty Adapter

This module integrates OpsOrch with PagerDuty using the PagerDuty REST API v2. It provides two adapters:
1.  **Incident Adapter**: Create, query, retrieve, and update PagerDuty incidents.
2.  **Service Adapter**: Discover and list PagerDuty services.

## Incident Adapter

The Incident Adapter implements the `incident.Provider` interface to manage PagerDuty incidents.

### Configuration

The incident adapter requires the following configuration:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `apiToken` | string | Yes | Your PagerDuty REST API v2 Token |
| `serviceID` | string | Yes | The PagerDuty Service ID where incidents will be created |
| `fromEmail` | string | Yes | Email address of a valid PagerDuty user (required for creating incidents) |
| `apiURL` | string | No | PagerDuty API URL (default: `https://api.pagerduty.com`) |
| `defaultSeverity` | string | No | Default severity for new incidents (default: `critical`) |
| `source` | string | No | Source identifier (default: `pagerduty`) |

### Capabilities

- **Create**: Creates new PagerDuty incidents (`POST /incidents`).
- **Get**: Retrieves incident details (`GET /incidents/{id}`).
- **Query**: Searches incidents with filters (`GET /incidents`).
- **Update**: Updates incident fields and status (`PUT /incidents/{id}`).
- **Timeline**: Retrieves (`GET /incidents/{id}/log_entries`) and appends (`POST /incidents/{id}/notes`) timeline entries.

### Mappings

**Severity to Urgency:**
- `critical`, `sev1`, `p1` → `high`
- `high`, `sev2`, `p2` → `high`
- `medium`, `sev3`, `p3` → `low`
- `low`, `sev4`, `p4` → `low`

**Status Mapping:**
- `open`, `triggered` → `triggered`
- `acknowledged`, `investigating` → `acknowledged`
- `resolved`, `closed` → `resolved`

### Query Filtering

The incident adapter supports the following query filters:

**Supported via PagerDuty API:**
- `Statuses` → maps to `statuses[]` parameter
- `Severities` → maps to `urgencies[]` parameter
- `Scope.Service` → queries PagerDuty services by canonical name, extracts IDs, maps to `service_ids[]`
- `Scope.Team` → queries PagerDuty teams by canonical name, extracts IDs, maps to `team_ids[]`
- `Metadata["service_id"]` → maps directly to `service_ids[]` parameter (PagerDuty service ID)
- `Metadata["team_id"]` → maps directly to `team_ids[]` parameter (PagerDuty team ID)
- `Metadata["incident_key"]` → maps to `incident_key` parameter

**Not Supported:**
- `Scope.Environment` - PagerDuty does not have a native environment concept
- `Query` (free-text search) - PagerDuty incidents API does not support full-text search

**Service ID Filtering Behavior:**
- The configured `serviceID` is only used for creating incidents, not for querying
- To filter queries by service, explicitly use `Scope.Service` or `Metadata["service_id"]`
- Queries without service filters will return incidents across all services (subject to API token permissions)

**Note:** `Scope` fields trigger additional API calls to translate canonical names to PagerDuty IDs. Use `Metadata` fields with known IDs for better performance.

---

## Service Adapter

The Service Adapter implements the `service.Provider` interface to discover PagerDuty services.

### Configuration

The service adapter requires the following configuration:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `apiToken` | string | Yes | Your PagerDuty REST API v2 Token |
| `apiURL` | string | No | PagerDuty API URL (default: `https://api.pagerduty.com`) |
| `source` | string | No | Source identifier (default: `pagerduty`) |

### Capabilities

- **Query**: List all services or filter by name (`GET /services`).

### Query Filtering

The service adapter supports the following query filters:

**Supported via PagerDuty API:**
- `Name` → maps to `query` parameter (fuzzy name search)
- `Scope.Team` → queries PagerDuty teams by canonical name, extracts IDs, maps to `team_ids[]`
- `Metadata["team_id"]` → maps directly to `team_ids[]` parameter (PagerDuty team ID)

**Not Supported:**
- `Scope.Service`, `Scope.Environment` - Not applicable for service queries
- `Tags` - PagerDuty services do not have a direct key-value tag system in the API

**Note:** `Scope.Team` triggers an additional API call to translate the team name to PagerDuty team IDs. Use `Metadata["team_id"]` with known IDs for better performance.

---

## Metadata Mapping

The adapter enriches the standard OpsOrch schema with PagerDuty-specific details in the `metadata` field.

### Incident Metadata
| Field | Description |
|-------|-------------|
| `source` | Always "pagerduty" |
| `incident_key` | The PagerDuty incident key (deduplication key) |
| `service_id` | ID of the affected service |
| `service_url` | Link to the service in PagerDuty UI |
| `html_url` | Direct link to the incident in PagerDuty UI |
| `last_status_change_at` | Timestamp of the last status change |
| `assignments` | List of assignees (includes `id`, `name`, `html_url`) |

### Service Metadata
| Field | Description |
|-------|-------------|
| `source` | Always "pagerduty" |
| `summary` | Brief summary of the service |
| `description` | Full description of the service |
| `status` | Current status of the service (active/warning/critical) |
| `html_url` | Direct link to the service in PagerDuty UI |
| `alert_creation` | How alerts are created (e.g., "create_incidents") |
| `escalation_policy` | Details of the escalation policy (id, summary) |
| `teams` | List of associated teams (id, summary) |

---

## Development

### Project Structure

```
opsorch-pagerduty-adapter/
├── common/                      # Shared utilities
│   └── lookup.go               # Service/Team name → ID lookups
├── incident/                    # Incident adapter
│   ├── pagerduty_provider.go
│   └── pagerduty_provider_test.go
├── service/                     # Service adapter
│   ├── pagerduty_provider.go
│   └── pagerduty_provider_test.go
├── cmd/
│   ├── incidentplugin/         # Incident plugin entrypoint
│   └── serviceplugin/          # Service plugin entrypoint
└── integ/                      # Integration tests
    ├── incident.go
    └── service.go
```

### Key Components

**Common Package (`common/`):**
- `LookupServiceIDsByName`: Queries PagerDuty services by canonical name and returns matching service IDs
- `LookupTeamIDsByName`: Queries PagerDuty teams by canonical name and returns matching team IDs

These functions are shared by both incident and service adapters to translate `Scope.Service` and `Scope.Team` filters.

### Building

```bash
make plugin
```
This builds both `bin/incidentplugin` and `bin/serviceplugin`.

### Testing

**Unit Tests:**
```bash
make test
```

**Integration Tests:**
To run integration tests against a real PagerDuty account:

```bash
# Service Adapter Tests
export PAGERDUTY_API_TOKEN="your-token"
make integ-service

# Incident Adapter Tests
export PAGERDUTY_API_TOKEN="your-token"
export PAGERDUTY_SERVICE_ID="PXXXXXX"
export PAGERDUTY_FROM_EMAIL="user@example.com"
make integ-incident

# Run All Integration Tests
make integ
```

**Integration Test Requirements:**
- A valid PagerDuty API token with read/write permissions
- A PagerDuty service ID where test incidents will be created
- A valid PagerDuty user email address (required for incident creation)

**Expected Integration Test Behavior:**
- Service tests will query your PagerDuty services and verify the adapter can list and filter them
- Incident tests will create, update, query, and clean up test incidents in the specified service
- Tests will verify timeline operations (reading log entries and adding notes)
- All test incidents are cleaned up after the test run completes

**Note:** Integration tests make real API calls and may create temporary incidents in your PagerDuty account. Ensure you're using a test service or are comfortable with test data in your production environment.

## Using In OpsOrch Core

You can use these adapters in OpsOrch Core by building the plugins and configuring them via environment variables.

**Incident Plugin:**
```bash
OPSORCH_INCIDENT_PLUGIN=/path/to/bin/incidentplugin
OPSORCH_INCIDENT_CONFIG='{"apiToken": "...", "serviceID": "...", "fromEmail": "..."}'
```

**Service Plugin:**
```bash
OPSORCH_SERVICE_PLUGIN=/path/to/bin/serviceplugin
OPSORCH_SERVICE_CONFIG='{"apiToken": "..."}'
```

## Plugin RPC Contract

OpsOrch Core communicates with the plugins over stdin/stdout using JSON-RPC.

**Request:**
```json
{
  "method": "incident.create",
  "config": { /* decrypted config */ },
  "payload": { /* method-specific body */ }
}
```

**Response:**
```json
{
  "result": { /* method-specific result */ },
  "error": "optional error message"
}
```

**Supported Methods:**
- `incident.query`, `incident.get`, `incident.create`, `incident.update`
- `incident.timeline.get`, `incident.timeline.append`
- `service.query`
