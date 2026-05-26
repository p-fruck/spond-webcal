# Development Guide

This is the primary development reference for this repository.

AGENTS.md is intentionally minimal and points here for full context.

## Stack

- Modern Go
- Echo v4
- GORM (SQLite + PostgreSQL)
- OpenAPI + oapi-codegen
- Testify + mockery

## Setup

1. Generate the API client from the OpenAPI spec:

```bash
just gen-client
```

2. Run quality checks:

```bash
just fmt
just lint
just test
```

or: `just ci`

## Development Workflow

Use TDD for all features:

1. Write a failing test.
2. Implement the smallest passing change.
3. Refactor while keeping tests green.

Keep commits small and focused.

## Rules

1. openapi.yaml is the API source of truth.
2. Do not manually edit generated files in internal/api.
3. Wrap errors with context.
4. Prefer interfaces for dependencies.
5. Add tests for all non-trivial logic.

## Common Commands

```bash
just # print all commands
just gen-client
just fmt
just lint
just test
just test-coverage
just ci
```

## Project Structure

```text
cmd/spond-webcal/        entrypoint
internal/api/            generated OpenAPI client
internal/spond/          Spond API wrapper
internal/caldav/         CalDAV + iCal handling
internal/db/             models and persistence
internal/auth/           session/token handling
internal/web/            HTTP handlers and templates
migrations/              database migrations
tests/                   unit, integration, fixtures
```

## Adding a New Spond Endpoint

1. Update openapi.yaml.
2. Run just gen-client.
3. Wrap generated client usage in internal/spond.
4. Add unit tests (and integration tests when needed).

## Definition of Done

- Tests pass.
- Lint passes.
- Formatting is clean.
- Changes are scoped and documented in commit message.