# syntax=docker/dockerfile:1.7

# -------- Builder --------
ARG GO_VERSION=1.24
FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /src
RUN apk add --no-cache git ca-certificates

# Cache modules first
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy the rest of the source
COPY . .

ARG TARGETOS
ARG TARGETARCH
ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/app


# -------- Runtime --------
FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app

# Binary
COPY --from=builder /out/app /app/app

# Runtime assets
COPY templates ./templates
COPY static ./static

# Create data directory with write permissions
# Note: distroless doesn't have shell, so we create it at runtime
# But we'll use a volume mount in production
ENV DATA_FILE=/app/data/complaints.json

# Default port (can be overridden by PORT env)
ENV PORT=8045
EXPOSE 8045

USER nonroot
ENTRYPOINT ["/app/app"]

