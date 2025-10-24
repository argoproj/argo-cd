# Azure AD Groups Overflow Resolution Implementation Plan

## Overview

Implement support for Azure AD groups overflow resolution to handle users with more than 200 group memberships. When Azure AD indicates overflow via `_claim_names` and `_claim_sources` in the ID token, fetch the complete group list (up to 1,000) using Microsoft Graph API.

## Current State Analysis

**Existing Infrastructure:**

- OIDC authentication in `util/oidc/oidc.go` with `HandleCallback()` processing ID tokens
- Access token caching with encryption in `GetUserInfo()` (lines 676-798)
- Group extraction via `GetScopeValues()` in `util/jwt/jwt.go` (lines 49-74)
- Azure config support in `util/settings/settings.go` with `AzureOIDCConfig` struct (lines 223-225)
- UserInfo endpoint group fetching in `SetGroupsFromUserInfo()` (lines 643-674)

**Key Files:**

- `util/oidc/oidc.go` - Main OIDC client implementation
- `util/settings/settings.go` - Configuration structures
- `util/jwt/jwt.go` - JWT claims parsing
- `server/server.go` - Server-side claims processing
- `docs/operator-manual/user-management/microsoft.md` - Azure AD documentation

## Implementation Steps

### 1. Extend Configuration (util/settings/settings.go)

Update `AzureOIDCConfig` struct to add new settings:

```go
type AzureOIDCConfig struct {
    UseWorkloadIdentity              bool `json:"useWorkloadIdentity,omitempty"`
    EnableGroupsOverflowResolution   bool `json:"enableGroupsOverflowResolution,omitempty"`  // New
    MaxGroupsLimit                   int  `json:"maxGroupsLimit,omitempty"`                  // New
}
```

Default values:

- `EnableGroupsOverflowResolution`: `true` (enabled by default for Azure)
- `MaxGroupsLimit`: `1000`

### 2. Create Azure Graph API Client (new file: util/oidc/azure_graph.go)

Create helper functions for Graph API interaction:

**Key Functions:**

- `detectOverflow(claims jwt.MapClaims) bool` - Check for `_claim_names` and `_claim_sources`
- `validateOverflowIndicators(claims jwt.MapClaims) error` - Validate overflow format
- `hasUserReadScope(accessToken string) bool` - Parse access token JWT and check for `User.Read` in `scp` claim
- `fetchGroupsFromGraphAPI(ctx context.Context, accessToken string, timeout time.Duration) ([]string, error)` - Call Microsoft Graph API
- `resolveAzureGroupsOverflow(ctx context.Context, idTokenClaims jwt.MapClaims, accessToken string, config *AzureOIDCConfig) ([]string, error)` - Main orchestration function

**Graph API Details:**

- Endpoint: `POST https://graph.microsoft.com/v1.0/me/getMemberGroups`
- Request body: `{"securityEnabledOnly": true}`
- Headers: `Authorization: Bearer {access_token}`, `Content-Type: application/json`
- Timeout: 5 seconds
- Error handling: All failures log and return empty groups (non-blocking)

### 3. Integrate Overflow Resolution into OIDC Callback (util/oidc/oidc.go)

Modify `HandleCallback()` function (around line 408-529):

**Integration Point:** After ID token verification and claims extraction (around line 480-485), before setting the auth cookie (around line 508-519)

**Flow:**

```
1. Existing: Verify ID token → Extract claims
2. NEW: Check if Azure provider AND overflow enabled
3. NEW: Call resolveAzureGroupsOverflow() with ID token claims and access token
4. NEW: If groups resolved, merge into claims["groups"]
5. Existing: Encrypt and cache access token
6. Existing: Set auth cookie with enhanced claims
```

**Error Handling:**

- Graph API failures do not block authentication
- Log warnings/errors appropriately
- Continue with empty groups array if resolution fails

### 4. Update SetGroupsFromUserInfo Integration (util/oidc/oidc.go)

Modify `SetGroupsFromUserInfo()` (lines 643-674) to integrate with overflow resolution:

**Priority Order:**

1. Check if groups already in claims (inline or from overflow resolution) → use those
2. If UserInfo endpoint enabled → try UserInfo endpoint
3. If Azure overflow enabled and detected → try Graph API resolution
4. Continue with empty groups if all fail

This maintains backward compatibility while adding the new capability.

### 5. Add Utility Functions (util/jwt/jwt.go or new util/azure/)

Add helper functions for JWT parsing and claim validation:

**Functions:**

- Parse claims as JSON strings (for `_claim_names` which has `ValueType` = `"JSON"`)
- Validate claim structure (check "groups" is only key)
- Safe claim extraction with type checking

### 6. Add Logging and Monitoring

**Log Levels:**

- **Info:** Overflow detected, starting resolution, successful resolution with count
- **Warning:** Missing User.Read scope, exceeds max limit (>1000), unexpected claim structure, existing groups claim present
- **Error:** Graph API failures (5xx, timeout, network errors), JSON parse errors

**Example Log Messages:**

```
INFO: Azure AD groups overflow detected for user {sub}, resolving via Graph API
INFO: Successfully resolved {count} groups for user {sub}
WARNING: User {sub} has {count} groups, exceeds limit of {maxLimit}, discarding all groups
ERROR: Failed to resolve groups via Graph API for user {sub}: {error}
```

### 7. Update Documentation (docs/operator-manual/user-management/microsoft.md)

Add new section explaining:

- What groups overflow is and when it occurs (200+ groups)
- How the automatic resolution works
- Configuration options (`enableGroupsOverflowResolution`, `maxGroupsLimit`)
- Prerequisites (User.Read scope must be granted)
- Troubleshooting common issues

**Configuration Example:**

```yaml
oidc.config: |
  name: Azure
  issuer: https://login.microsoftonline.com/{tenant_id}/v2.0
  clientID: {client_id}
  clientSecret: $oidc.azure.clientSecret
  azure:
    useWorkloadIdentity: false
    enableGroupsOverflowResolution: true  # Default: true
    maxGroupsLimit: 1000                  # Default: 1000
  requestedScopes:
    - openid
    - profile
    - email
    - User.Read  # Required for groups overflow resolution
```

### 8. Add Unit Tests

**Test Files:**

- `util/oidc/azure_graph_test.go` - Test Graph API functions
- `util/oidc/oidc_test.go` - Test integration in HandleCallback

**Test Scenarios:**

- Overflow detection with valid `_claim_names` and `_claim_sources`
- Overflow validation (single "groups" key, no existing groups claim)
- Access token scope checking (User.Read present/absent)
- Graph API call success with various group counts (< 1000, = 1000, > 1000)
- Graph API failures (401, 403, 429, 5xx, timeout, network errors)
- Integration with existing groups claim handling
- Configuration variations (overflow disabled, custom max limit)

### 9. Add Integration Tests

Test end-to-end flow with mocked Azure AD:

- Mock OIDC provider returning overflow indicators
- Mock Graph API endpoint
- Verify groups are correctly added to user session
- Verify RBAC works with resolved groups

## Security Considerations

1. **Access Token Security:** Continue using existing encrypted cache, no additional measures needed
2. **Scope Validation:** Always verify User.Read scope before Graph API calls
3. **Non-Blocking Auth:** Graph API failures never prevent authentication
4. **Deterministic Behavior:** Discard all groups if count > max limit (don't take first N)
5. **Error Exposure:** Never expose internal errors to end users
6. **Audit Logging:** Log all Graph API calls for security auditing

## Configuration Values

| Setting | Default | Description |

|---------|---------|-------------|

| `enableGroupsOverflowResolution` | `true` | Enable automatic overflow resolution for Azure AD |

| `maxGroupsLimit` | `1000` | Maximum groups to add (discard all if exceeded) |

| Graph API Timeout | `5s` | Timeout for Graph API calls |

## Rollout Strategy

1. **Phase 1:** Implement core functionality with feature flag disabled by default
2. **Phase 2:** Enable by default for new Azure AD configurations
3. **Phase 3:** Document migration path for existing Azure AD users

## Files to Modify/Create

**New Files:**

- `util/oidc/azure_graph.go` - Graph API client and overflow resolution logic
- `util/oidc/azure_graph_test.go` - Unit tests

**Modified Files:**

- `util/settings/settings.go` - Add AzureOIDCConfig fields
- `util/oidc/oidc.go` - Integrate overflow resolution in HandleCallback()
- `docs/operator-manual/user-management/microsoft.md` - Add documentation
- Potentially `util/jwt/jwt.go` - Add helper functions if needed

## Success Criteria

- Azure AD users with 200+ groups can authenticate successfully
- Groups are correctly resolved via Graph API (up to 1,000)
- RBAC policies work with resolved groups
- Graph API failures do not block authentication
- All security considerations are addressed
- Comprehensive test coverage (unit + integration)
- Documentation is clear and complete