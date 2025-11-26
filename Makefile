GO ?= go
GOCACHE ?= $(PWD)/.gocache
GOMODCACHE ?= $(PWD)/.gocache/mod
CACHE_ENV = GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE)

.PHONY: all fmt test build plugin integ integ-incident integ-service clean

all: test

fmt:
	$(GO)fmt -w .

test:
	$(CACHE_ENV) $(GO) test ./...

build:
	$(CACHE_ENV) $(GO) build ./...

plugin:
	$(CACHE_ENV) $(GO) build -o bin/incidentplugin ./cmd/incidentplugin
	$(CACHE_ENV) $(GO) build -o bin/serviceplugin ./cmd/serviceplugin

integ-incident:
	@if [ -z "$$PAGERDUTY_API_TOKEN" ]; then \
		echo "Error: PAGERDUTY_API_TOKEN environment variable is required"; \
		exit 1; \
	fi
	@if [ -z "$$PAGERDUTY_SERVICE_ID" ]; then \
		echo "Error: PAGERDUTY_SERVICE_ID environment variable is required for incident tests"; \
		exit 1; \
	fi
	@if [ -z "$$PAGERDUTY_FROM_EMAIL" ]; then \
		echo "Error: PAGERDUTY_FROM_EMAIL environment variable is required for incident tests"; \
		exit 1; \
	fi
	$(CACHE_ENV) $(GO) run ./integ/incident.go

integ-service:
	@if [ -z "$$PAGERDUTY_API_TOKEN" ]; then \
		echo "Error: PAGERDUTY_API_TOKEN environment variable is required"; \
		exit 1; \
	fi
	$(CACHE_ENV) $(GO) run ./integ/service.go

integ: integ-service integ-incident

clean:
	rm -rf $(GOCACHE) bin
