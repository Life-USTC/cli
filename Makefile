VERSION ?= dev
OPENAPI_SERVER_DIR ?= ../server
SERVER_COMMIT ?=
LDFLAGS := -ldflags "-X github.com/Life-USTC/CLI/internal/cmd/root.version=$(VERSION)"

.PHONY: build clean test test-scripts lint vet install generate sync-openapi check-openapi-provenance check-openapi-reachability check-openapi-sync

build: check-openapi-provenance generate
	go build $(LDFLAGS) -o life-ustc ./cmd/life-ustc

clean:
	rm -f life-ustc
	rm -rf dist/

test: test-scripts
	go test -race ./...

test-scripts:
	./scripts/openapi-contract.test.sh

lint:
	golangci-lint run ./...

vet:
	go vet ./...

install: check-openapi-provenance generate
	go install $(LDFLAGS) ./cmd/life-ustc

generate:
	go tool oapi-codegen -config api/oapi-codegen.yaml api/openapi.json
	go run ./internal/cmd/apicmd/genpaths

sync-openapi:
	./scripts/openapi-contract sync "$(OPENAPI_SERVER_DIR)" "$(SERVER_COMMIT)"

check-openapi-provenance:
	./scripts/openapi-contract verify

check-openapi-reachability:
	./scripts/openapi-contract verify-reachable "$(OPENAPI_SERVER_DIR)"

check-openapi-sync:
	./scripts/openapi-contract verify-source "$(OPENAPI_SERVER_DIR)"
