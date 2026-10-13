# syntax=docker/dockerfile:1

# =============================================================================
# Builder
# =============================================================================
FROM golang:1.27-alpine AS builder

ARG VERSION=dev

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Download modules first so source changes do not invalidate the dependency layer
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /out/server ./cmd/server-template

# =============================================================================
# Runtime
# =============================================================================
FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/server /app/server
COPY config/config_demo.yaml /app/config/config.yaml

WORKDIR /app
EXPOSE 8080 9090

ENTRYPOINT ["/app/server"]
