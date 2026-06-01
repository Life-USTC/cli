VERSION ?= dev
OPENAPI_SOURCE ?= ../server-nextjs/public/openapi.generated.json
LDFLAGS := -ldflags "-X github.com/Life-USTC/CLI/internal/cmd/root.version=$(VERSION)"

.PHONY: build clean test lint install generate sync-openapi check-openapi-sync

build:
	go build $(LDFLAGS) -o life-ustc ./cmd/life-ustc

clean:
	rm -f life-ustc
	rm -rf dist/

test:
	go test ./...

lint:
	golangci-lint run ./...

install:
	go install $(LDFLAGS) ./cmd/life-ustc

generate:
	go tool oapi-codegen -config api/oapi-codegen.yaml api/openapi.json
	go run ./internal/cmd/apicmd/genpaths

sync-openapi:
	cp $(OPENAPI_SOURCE) api/openapi.json

check-openapi-sync:
	cmp -s $(OPENAPI_SOURCE) api/openapi.json
