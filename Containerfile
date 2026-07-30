FROM docker.io/library/golang:1.25-alpine AS builder

WORKDIR /src

# Install CA certificates once so we can copy them into the scratch image.
RUN apk add --no-cache ca-certificates

# Cache dependency resolution first.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a static binary.
COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags='-s -w' -o /out/spond-webcal ./cmd/spond-webcal

FROM scratch

# TLS trust store for outbound HTTPS calls.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Application binary.
COPY --from=builder /out/spond-webcal /spond-webcal

# Run as non-root numeric user.
USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/spond-webcal"]
