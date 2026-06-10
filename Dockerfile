# Multi-stage Dockerfile for Go version of NextDNS IP Updater
# Produces a minimal, secure container using distroless base (~10MB total)

# Build stage
FROM golang:1.23-bookworm AS builder

# Accept build arguments for version information
ARG VERSION=dev
ARG BUILD_TIME=unknown
ARG TARGETOS
ARG TARGETARCH

WORKDIR /build

# Copy Go module files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY *.go ./

# Build static binary with version information injected
# CGO_ENABLED=0 ensures a fully static binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build \
    -ldflags "-w -s -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}" \
    -o nextdns-ip-updater \
    .

# Runtime stage using distroless
FROM gcr.io/distroless/static-debian12:nonroot

# Copy the static binary from builder
COPY --from=builder /build/nextdns-ip-updater /usr/local/bin/nextdns-ip-updater

# Set environment variables with defaults
ENV UPDATE_INTERVAL_SECONDS=300

# Run as non-root user (distroless nonroot user is UID 65532)
USER nonroot:nonroot

# Execute the binary
ENTRYPOINT ["/usr/local/bin/nextdns-ip-updater"]
