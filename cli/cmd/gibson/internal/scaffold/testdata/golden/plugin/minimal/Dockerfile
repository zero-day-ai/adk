# Stage 1: build — compiles the plugin binary with CGO disabled.
FROM golang:1.25-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/byte-identity .

# Stage 2: runtime — minimal image for the pod runtime mode.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

COPY --from=builder /out/byte-identity /byte-identity
COPY plugin.yaml /plugin.yaml

EXPOSE 8080

ENTRYPOINT ["/byte-identity"]
