BINARY := demo-tool

.PHONY: proto build test register run image

## proto: regenerate Go bindings from api/proto/demo-tool/v1/demo-tool.proto.
## Requires buf (https://buf.build/docs/installation), protoc-gen-go, and
## protoc-gen-go-grpc on PATH.
proto:
	buf generate

## build: compile the tool binary.
build:
	CGO_ENABLED=0 go build -o $(BINARY) .

## test: run unit tests.
test:
	go test ./...

## register: write OAuth2 credentials issued by the dashboard.
## Run `gibson component register` and paste the enroll_command's flags.
register:
	@echo "Run: gibson component register --client-id <id> --client-secret - --gibson-url <url>"
	@echo "Or paste the enroll_command from the Gibson dashboard."

## run: run the compiled binary; reads credentials from ~/.gibson/tool/credentials.
run: build
	gibson component run

## image: build the container image.
image:
	docker build -t $(BINARY):0.1.0 .
