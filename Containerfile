FROM docker.io/library/golang:1.25-alpine AS builder

WORKDIR /src

# currently required for sqlite implementation
RUN apk add --no-cache gcc musl-dev
ENV CGO_ENABLED=1

# Install CA certificates once so we can copy them into the scratch image.
RUN apk add --no-cache ca-certificates

# Cache dependency resolution first.
COPY go.mod go.sum ./
RUN go mod download

COPY openapi.yaml ./
RUN go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -package api \
            -generate types,client \
            -o internal/api/client.gen.go \
            openapi.yaml


# Copy source and build a static binary.
COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build \
      -trimpath -ldflags='-s -w -linkmode external -extldflags "-static"' \
      -tags sqlite_omit_load_extension \
      -o /out/spond-webcal ./cmd/spond-webcal

FROM scratch

# TLS trust store for outbound HTTPS calls.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Application binary.
COPY --from=builder /out/spond-webcal /spond-webcal

# Run as non-root numeric user.
USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/spond-webcal"]
