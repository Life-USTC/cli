VERSION ?= dev
LDFLAGS := -ldflags "-X github.com/Life-USTC/CLI/internal/cmd/root.version=$(VERSION)"

.PHONY: build clean test lint install generate

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
