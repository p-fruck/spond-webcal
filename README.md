# Spond Web Calendar

A multi-tenant web-based calendar for Spond, written in Go, that bridges the proprietary Spond app with standard calendar tooling (ICS/CalDAV).

Spond API integration based on [Olen's Spond](https://github.com/Olen/Spond). API definition in [openapi.yaml](./openapi.yaml), go client is generated via OpenAPI generator.

## Motivation

I like to avoid any proprietary software and wanted to avoid installing Spond on my mobile phone. So I thought: Why not create a small service that transforms the Spond events into some standardized calendar format.

Additionally, I wanted to share my Spond Calender with other people (outside the Spond group) so they know when I am away. Hence, this implementation sharing certain calender events with a given RSVP via access-tokens as well.

**Is this vibe coded?** Yes. Unfortunately, I lack the freetime to develop an open source replacement for every proprietary piece of software in my life, hence I asked the AI for help.

**Is this code secure?** Probably not. Think twice before exposing software written by some other guy or robot to the internet.

## Features

- [x] **Web UI**
  - [x] Sign-in to the Web UI using your Spond Credentials
  - [x] Spond Event Listing
    - [x] Upcoming events list
    - [x] Past events in collapsible section
    - [x] Filter events by group and RSVP
- [ ] Full multi-account UX in web app
- [x] **CalDAV Baseline**
  - [x] CalDAV endpoints mounted (`/caldav`)
  - [x] Persistence-backed resource store
  - [x] ETag + If-Match conflict semantics
- [x] **iCalendar (ICS) feed endpoint**
  - [x] ICS export via HTTPS with group/RSVP filters
  - [x] Token-based sharing of groups
- [ ] **Write-Back to Spond RSVP changes**

## Quick Start

```bash
just gen-client
SPOND_WEBCAL_COOKIE_SECRET=mysecret just run
```

### Configuration

Environment variables:

```bash
SPOND_WEBCAL_ADDR=":8080"                                   # Listen address
SPOND_WEBCAL_DB_URL="file:./app.db"                         # Database URL (SQLite file or postgres://)
SPOND_WEBCAL_COOKIE_SECRET="<required-random-secret>"       # REQUIRED for startup
SPOND_WEBCAL_LOG_LEVEL="info"                               # Log level
SPOND_WEBCAL_SPOND_BASE_URL="https://api.spond.com/core/v1" # Spond API base URL
```

See the [developer docs](docs/dev.md) for detailed architecture and developer guide.

## Known Limitations

- **2FA**: Spond SDK doesn't support two-factor authentication. Disable 2FA or create a separate account without 2FA.
- **Chat**: Messaging/chat are not planned
- **Event Creation**: Can't create events via CalDAV
