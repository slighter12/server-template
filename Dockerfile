# Build Stage
# Install tools required for project
# Run `docker build --no-cache .` to update dependencies
FROM golang:1.23-alpine AS builder

ARG VERSION
ARG BUILT
ARG GIT_COMMIT
ARG IMAGE_NAME

RUN apk update && \
    apk add --no-cache git curl

WORKDIR /app

COPY . .

RUN go mod download

ENV FLAG="-s -w -X main.Version=${VERSION} -X main.Built=${BUILT} -X main.GitCommit=${GIT_COMMIT}"

RUN GOOS=linux GOARCH=amd64 go build \
    -ldflags "$FLAG" \
    -o /app/${IMAGE_NAME} ./cmd/${IMAGE_NAME}

# Final Stage
# This results in a single layer image
FROM alpine:latest AS final

ARG IMAGE_NAME

WORKDIR /usr/local/bin

RUN apk update && \
    apk upgrade && \
    apk add --no-cache ca-certificates

ADD https://github.com/golang/go/raw/master/lib/time/zoneinfo.zip /usr/local/go/lib/time/zoneinfo.zip

COPY --from=builder /app/${IMAGE_NAME} ./${IMAGE_NAME}
COPY --from=builder /app/configs/config.yaml ./config.yaml

ENTRYPOINT ["/usr/local/bin/${IMAGE_NAME}"]
