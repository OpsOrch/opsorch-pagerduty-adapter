# OpsOrch PagerDuty Adapter

This module is the PagerDuty-flavoured version of the OpsOrch adapter starter. It keeps the in-memory behavior from the starter so it can be compiled, tested, and wired into OpsOrch Core immediately, while giving you a scaffold to swap in real PagerDuty API calls.

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
The stub provider accepts (and defaults) the fields shown below. Swap/extend these when adding real PagerDuty integration settings.

```json
{
  "source": "pagerduty",
  "defaultSeverity": "sev3",
  "apiToken": "pd_api_token",
  "apiURL": "https://api.pagerduty.com"
}
```

`apiToken` is required; `apiURL` defaults to PagerDuty's production endpoint but can point at mocks during development.

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
