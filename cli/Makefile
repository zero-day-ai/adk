BINARY := gibson

.PHONY: build test test-integration update-golden lint vet install

## build: compile the gibson CLI binary into ./bin/gibson.
build:
	@mkdir -p bin
	go build -o bin/$(BINARY) ./cmd/gibson

## install: go install the gibson binary into $GOPATH/bin.
install:
	go install ./cmd/gibson

## test: run unit tests (excludes integration tests).
test:
	go test ./...

## test-integration: run integration tests that build scaffolded
## components against the Go module proxy (and buf, when available).
## Requires network access; gated behind the `integration` build tag
## so it doesn't run as part of the default `go test ./...`.
test-integration:
	go test -tags=integration ./...

## update-golden: regenerate scaffold golden files. Run after an
## intentional template change.
update-golden:
	go test -update -run TestRender_PluginGoldenFiles ./cmd/gibson/internal/scaffold

## vet: go vet on the entire CLI tree.
vet:
	go vet ./...

## lint: golangci-lint if it's on PATH; otherwise no-op.
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not on PATH — skipping"; \
	fi
