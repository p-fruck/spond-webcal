# Spond WebCAL Server

A multi-tenant CalDAV/WebCal server written in Go that bridges the proprietary Spond app with standard calendar protocols. Sync your Spond events to any CalDAV/WebCal-compatible client (DAVx5, ICSx5, iOS Calendar, etc.) and update attendance directly from your phone.

## Features

- [ ] **Bidirectional Sync**: Accept/decline events from your phone and sync back to Spond
- [ ] **Multi-Tenant**: Sign in with multiple Spond accounts independently
- [ ] **CalDAV + WebCal**: Support for both protocols
  - [ ] **CalDAV**: Bidirectional (read+write); update attendance via DAVx5/ICSx5 on Android
  - [ ] **WebCal**: Read-only iCalendar subscription; suitable for iOS Calendar and others
- [ ] **Token-Based Sharing**: Create shareable calendar links with configurable scopes
  - [ ] Public/private calendars
  - [ ] Read-only or read-write access
  - [ ] Filter by event status (accepted/declined/tentative)
- [ ] **Web UI**: Sign-in dashboard + account overview + token management

## Quick Start

```bash
just gen-client
just run
```

### Configuration

Environment variables (all optional, defaults shown):

```bash
SPOND_WEBCAL_ADDR=":8080"              # Listen address
SPOND_WEBCAL_DB_URL="file:./app.db"    # Database URL (SQLite file or postgres://)
SPOND_WEBCAL_COOKIE_SECRET=""          # Secret for session cookies (MUST set in production)
SPOND_WEBCAL_LOG_LEVEL="debug"         # Enable debug logging
```

See the [developer docs](docs/dev.md) for detailed architecture and developer guide.

## Supported Calendar Apps

| App | Platform | Protocol | Write-Back |
|-----|----------|----------|-----------|
| DAVx5 | Android | CalDAV | ✓ Yes |
| ICSx5 | Android | CalDAV | ✓ Yes |
| Apple Calendar | iOS/macOS | WebCal | ✗ No |
| Thunderbird | Desktop | CalDAV | ✓ Yes |
| GNOME Calendar | Linux | CalDAV | ✓ Yes |

## Known Limitations

- **2FA**: Spond SDK doesn't support two-factor authentication. Disable 2FA or create a separate account without 2FA.
- **Chat**: Messaging/chat are not planned
- **Event Creation**: Can't create events via CalDAV
