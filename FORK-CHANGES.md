# OpenNHP Fork Changes

This document describes the modifications made to the official [OpenNHP](https://github.com/OpenNHP/opennhp) repository for the Obsession Cottonwood integration.

## Overview

Our fork implements **True NHP Invisibility** with Discord-based authentication. Key modifications:

1. **Discord Authentication Plugin** - Custom Go plugin for Discord-gated access
2. **One-Time Invitation Tokens** - Clickable links instead of manual key entry
3. **iptables Integration** - NHP-AC manages per-IP firewall rules
4. **Silent Failures** - No error responses (HTTP 444) for true invisibility

## Authentication Flow

```text
┌─────────────────────────────────────────────────────────────────────┐
│  1. Discord /nhp-access command                                      │
│     └── Bot generates one-time invitation token (6hr expiry)        │
│     └── Returns clickable "Access Cottonwood" button                │
│                                                                      │
│  2. User clicks button → GET /knock?t=TOKEN                          │
│     └── NHP Server validates token (exists, not used, not expired)  │
│     └── Marks token as used (one-time)                              │
│     └── Triggers NHP-AC to whitelist user's IP                      │
│     └── Sets cookies for Discord ID identification                  │
│     └── Redirects to application                                    │
│                                                                      │
│  3. Application access                                               │
│     └── iptables allows traffic from whitelisted IP                 │
│     └── Access open for 5 minutes (DefaultOpenTime)                 │
└─────────────────────────────────────────────────────────────────────┘
```

## Modified Files

### Added: Discord Authentication Plugin

**Location:** `examples/discord_auth_plugin/`

| File | Purpose |
|------|---------|
| `main.go` | Plugin implementation with knock/validate actions |
| `etc/config.toml` | Plugin configuration |

**Key Functions:**

```go
// knockWithInviteToken - Primary authentication method
// Validates one-time token, triggers AC whitelist, sets cookies
func knockWithInviteToken(ctx, req, res, helper) (*ServerKnockAckMsg, error)

// validateAndConsumeInviteToken - Token validation
// Checks: exists, not used, not expired, NHP key not revoked
func validateAndConsumeInviteToken(token string) (bool, string, string)

// Deprecated functions (return HTTP 444):
// - showLoginPage() - No visible login page
// - authWithNhpKey() - No manual key entry
```

### Added: Docker Build Files

**Location:** `docker/`

| File | Purpose |
|------|---------|
| `Dockerfile.base` | OpenNHP base image with build dependencies |
| `Dockerfile.server` | NHP Server with Discord plugin |
| `Dockerfile.ac` | NHP Access Controller |

### Modified: Build Configuration

**Changes to support plugin architecture:**

- Added plugin build step to server Dockerfile
- Configured shared volume for keys.db between Discord bot and NHP server
- Added NET_ADMIN capability for iptables manipulation

## Database Schema

The Discord bot and NHP server share a SQLite database (`keys.db`):

```sql
-- NHP keys (internal, not shown to users)
CREATE TABLE nhp_keys (
    id TEXT PRIMARY KEY,
    discord_id TEXT NOT NULL,
    discord_username TEXT,
    nhp_key TEXT NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME,
    revoked BOOLEAN DEFAULT FALSE,
    revoked_at DATETIME,
    revoked_reason TEXT
);

-- One-time invitation tokens
CREATE TABLE nhp_invite_tokens (
    token TEXT PRIMARY KEY,
    nhp_key_id TEXT NOT NULL,
    discord_id TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,
    used BOOLEAN DEFAULT FALSE,
    used_at DATETIME,
    used_ip TEXT,
    FOREIGN KEY (nhp_key_id) REFERENCES nhp_keys(id)
);
```

## NHP-AC Configuration

**Location:** `obsession-cottonwood/nhp-config/ac/etc/config.toml`

```toml
# Access Controller for Cottonwood
ACId = "cottonwood-ac"
DefaultIp = "172.28.0.11"
IpPassMode = 0              # Strict iptables blocking
FilterMode = 0              # iptables mode
DefaultOpenTime = 300       # 5 minute access window

# Protected resource
[[ProtectedHosts]]
ResourceId = "cottonwood"
Host = "172.28.0.20"        # Cottonwood container IP
Port = 3001
Protocol = "tcp"
```

## Why These Changes?

### Problem with Standard OpenNHP

The standard OpenNHP implementation requires:
1. Manual key copy/paste from Discord
2. A visible login page at a known URL
3. Error responses (401) that reveal the service exists

### Our Solution

| Aspect | Standard | Our Fork |
|--------|----------|----------|
| Key delivery | User copies key | Clickable button |
| Login page | Visible at /nhp-login | None (invisible) |
| Error responses | 401 with hints | HTTP 444 (silent close) |
| Access control | Cookie validation | iptables per-IP |
| Token lifetime | 30 days | 6 hours, one-time use |
| Link sharing | Shareable key | One-time, consumed on use |

### True Invisibility

Before a valid knock:
- Port 443: Only `/knock` responds (with valid token)
- Cottonwood port: Blocked by iptables (times out)
- Error responses: None (connection closes silently)

After a valid knock:
- User's IP whitelisted for 5 minutes
- Full application access
- Other IPs still blocked

## Upstream Compatibility

This fork maintains compatibility with the OpenNHP protocol while adding:
1. HTTP-based knock endpoint (in addition to UDP)
2. External token validation (Discord bot generates tokens)
3. Custom plugin architecture for authentication

We regularly pull upstream changes and rebase our modifications.

## Related Documentation

- [Cottonwood ARCHITECTURE.md](../obsession-cottonwood/docs/ARCHITECTURE.md)
- [Cottonwood README.md](../obsession-cottonwood/README.md)
- [Discord Bot nhp-access command](../obsession-discord/src/commands/nhp-access.ts)
