BINARY   := byte-identity-secret
MANIFEST := ./plugin.yaml
TOKEN    ?=

.PHONY: proto build test register run image

## proto: regenerate Go bindings from byte-identity-secret.proto.
## Requires buf (https://buf.build/docs/installation), protoc-gen-go,
## and protoc-gen-go-grpc on PATH.
proto:
	buf generate

## build: compile the plugin binary (CGO_ENABLED=0).
build:
	CGO_ENABLED=0 go build -o $(BINARY) .

## test: run unit tests.
test:
	go test ./...

## register: first-time registration with the Gibson daemon.
## Usage: make register TOKEN=<bootstrap-token>
register:
	@if [ -z "$(TOKEN)" ]; then echo "TOKEN is required: make register TOKEN=<bootstrap-token>"; exit 1; fi
	gibson component register --token "$(TOKEN)"

## run: start the plugin via the CLI's process supervisor.
run: build
	gibson component run

## image: build the container image.
image:
	docker build -t $(BINARY):1.2.3 .
