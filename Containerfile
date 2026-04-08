FROM --platform=$BUILDPLATFORM registry.hub.docker.com/library/golang:1.25.9-alpine3.23 AS builder

ARG OPERATOR_VERSION=1.0.0-dev

RUN apk update && apk upgrade && \
    apk add --no-cache bash build-base

WORKDIR /opt/db-operator

# to reduce docker build time download dependency first before building
COPY go.mod .
COPY go.sum .
RUN go mod download

# build
COPY . .
ARG TARGETARCH
RUN GOOS=linux GOARCH=$TARGETARCH CGO_ENABLED=0 \
  go build \
  -ldflags="-X \"github.com/db-operator/db-operator/v2/internal/helpers/common.OperatorVersion=$OPERATOR_VERSION\"" \
  -tags build -o /usr/local/bin/db-operator cmd/main.go


FROM gcr.io/distroless/static
LABEL org.opencontainers.image.authors="Nikolai Rodionov<allanger@badhouseplants.net>"
COPY --from=builder /usr/local/bin/db-operator /usr/local/bin/db-operator
USER 1001
ENTRYPOINT ["/usr/local/bin/db-operator"]
