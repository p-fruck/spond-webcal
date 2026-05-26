---
title: Spond WebCAL Agent Guide
description: Minimal instructions for coding agents in this repository.
keywords: Go, CalDAV, OpenAPI, TDD
---

# Agent Guide

This file is intentionally short.

For full project context, workflow, testing conventions, and architecture details, read docs/dev.md first.

## Core Rules

1. Follow TDD: write tests first, implement minimal code, then refactor.
2. Do not edit generated code manually in internal/api.
3. Keep changes small and incremental.
4. Run checks before finishing: format/lint/test
5. Prefer clear error wrapping and dependency injection via interfaces.

## Project Facts

- Module: code.p-fruck.eu/spond-webcal
- Language: Go 1.22+
- API spec source of truth: openapi.yaml
- Main command shortcuts: Justfile

## If Unsure

Read docs/dev.md and follow it as the primary reference.