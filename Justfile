# Spond WebCAL Server - Just commands

# Show list of all commands
@help:
    just --list

# Build the application
build:
    CGO_ENABLED=1 go build -o bin/spond-webcal ./cmd/spond-webcal

# Generate Go client from OpenAPI spec
gen-client:
    @echo "Generating Go client from openapi.yaml..."
    go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -package api \
        -generate types,client \
        -o internal/api/client.gen.go \
        openapi.yaml
    @echo "✓ Client generated to internal/api/client.gen.go"

# Run the server locally (requires database setup)
run: build
    ./bin/spond-webcal

# Run database migrations
migrate-up:
    go run -tags 'sqlite' ./cmd/migrate up

# Rollback database migrations
migrate-down:
    go run -tags 'sqlite' ./cmd/migrate down

# Clean build artifacts
clean:
    rm -rf bin/
    rm -f coverage.out coverage.html
