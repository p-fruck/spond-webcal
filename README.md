# Spond Calendar

A multi-tenant CalDAV server with a web UI, written in Go, that bridges the proprietary Spond app with standard calendar tooling.

## Features

- [x] **Web UI + Auth Session**
  - [x] Dedicated sign-in page (`/signin`)
  - [x] Cookie-based authenticated session
  - [x] Events dashboard at `/`
  - [x] Profile page (`/profile`) and logout (`/logout`)
- [x] **Spond Event Listing**
  - [x] Upcoming events list
  - [x] Past events in collapsible section
  - [x] RSVP status mapping (accepted/declined/waiting/unconfirmed/unanswered/unknown)
- [x] **CalDAV Baseline**
  - [x] CalDAV endpoints mounted (`/caldav`)
  - [x] Persistence-backed resource store
  - [x] ETag + If-Match conflict semantics
- [ ] **Write-Back to Spond RSVP changes**
- [ ] **iCalendar (ICS) feed endpoint**
  - [ ] HTTP ICS feed
  - [ ] Optional `webcal://` link exposure for compatible clients
- [ ] **Token-based sharing links/scopes**
- [ ] **Full multi-account UX in web app**

## Quick Start

```bash
just gen-client
SPOND_WEBCAL_COOKIE_SECRET=mysecret just run
```

### Configuration

Environment variables:

```bash
SPOND_WEBCAL_ADDR=":8080"                           # Listen address
SPOND_WEBCAL_DB_URL="file:./app.db"                 # Database URL (SQLite file or postgres://)
SPOND_WEBCAL_COOKIE_SECRET="<required-random-secret>" # REQUIRED for startup
SPOND_WEBCAL_LOG_LEVEL="info"                       # Log level
SPOND_WEBCAL_SPOND_BASE_URL="https://api.spond.com/core/v1" # Spond API base URL
```

See the [developer docs](docs/dev.md) for detailed architecture and developer guide.

## Known Limitations

- **2FA**: Spond SDK doesn't support two-factor authentication. Disable 2FA or create a separate account without 2FA.
- **Chat**: Messaging/chat are not planned
- **Event Creation**: Can't create events via CalDAV
