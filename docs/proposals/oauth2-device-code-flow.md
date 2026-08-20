---
title: oauth2-device-code-flow
authors:
  - "@dkarpele"
sponsors: []
reviewers: []
approvers: []

creation-date: 2026-08-14
last-updated: 2026-08-14
---

# OAuth 2.0 Device Code Authorization Grant for the Argo CD CLI

## Summary

Add `--no-browser` flag to `argocd login` and `argocd relogin` that uses the
[OAuth 2.0 Device Code Authorization Grant (RFC 8628)](https://www.rfc-editor.org/rfc/rfc8628)
instead of the standard PKCE browser redirect flow.

## Motivation

`argocd login --sso` redirects the user to `http://localhost:8085/auth/callback`. This requires
a browser running on the same machine as the CLI. In cloud-based IDEs (Red Hat DevSpaces, Eclipse Che,
GitHub Codespaces) the CLI runs inside a remote container, so `localhost` is unreachable from the
user's browser and login fails.

The existing `--callback` flag does not solve this for users of the bundled Dex: the `argo-cd-cli`
static client in Dex hardcodes only `localhost` redirect URIs, and wildcard redirect URIs are a
security anti-pattern not supported by Dex. External OIDC providers can register arbitrary redirect
URIs, but dynamic workspace URLs (which include a unique username and workspace name) make
pre-registration impractical without wildcards.

### Goals

- `argocd login <server> --sso --no-browser` works from any environment without a locally reachable
  port.
- `argocd relogin --no-browser` refreshes an expired token the same way.
- Works with bundled Dex and external OIDC providers (Keycloak, Okta, Azure AD, etc.).
- No operator configuration required for standards-compliant providers.

### Non-Goals

- Automating browser interaction or injecting credentials.
- Modifying the PKCE flow or the `--callback` flag behaviour.

## Proposal

### Use cases

#### Use case 1
A developer working inside Red Hat DevSpaces wants to use the Argo CD CLI to manage applications.
They run `argocd login argocd.example.com --sso --no-browser`, open the printed URL in their local
browser, approve the request, and the CLI stores the token automatically.

### Implementation Details

**CLI (`cmd/argocd/commands/login.go`, `relogin.go`)**

- Add `--no-browser bool` flag to both commands.
- When set with `--sso`, call `oauth2LoginNoBrowser` instead of `oauth2Login`.
- Device authorization endpoint is resolved in priority order:
  1. `oauth2conf.Endpoint.DeviceAuthURL` — auto-discovered from `device_authorization_endpoint`
     in the OIDC discovery document (populated automatically by `go-oidc/v3`).
  2. `oidcSettings.GetDeviceURL()` — operator-configured fallback in `argocd-cm`.

**Dex configuration (`util/dex/config.go`)**

The `argo-cd-cli` static client in Dex is updated to include:
- `grantTypes: [authorization_code, urn:ietf:params:oauth:grant-type:device_code]`
- `/device/callback` added to `redirectURIs` (required by Dex internally during the device flow).

### Security Considerations

- The device code flow does not use redirect URIs visible to the user, eliminating the open-redirect
  risk that motivated the `argo-cd-cli` localhost-only whitelist.
- Device codes are short-lived (`expires_in`, typically 5–10 minutes) and single-use.
- The user code displayed on screen conveys no secret; the actual authorization token is never shown.
- Polling uses the server-specified `interval` to avoid triggering rate limits; `slow_down` responses
  further increase the interval.

### Risks and Mitigations

| Risk                                             | Mitigation                                                                              |
|--------------------------------------------------|-----------------------------------------------------------------------------------------|
| OIDC provider does not support device code grant | CLI exits with a clear error from `requestDeviceCode`; operator can check provider docs |
| User cancels on the provider side but CLI hangs  | CLI exits when `expired_token` is returned or `Ctrl+C` cancels the context              |

### Upgrade / Downgrade Strategy

- Purely additive: new flag, no changes to existing behaviour.
- Dex `redirectURIs` change is backward-compatible; existing clients are unaffected.
- Downgrading: `--no-browser` flag will not exist in older CLI versions; users fall back to `--sso`.

## Alternatives

**`--callback` flag** (PR #22784): Allows overriding the redirect URI. Does not work with bundled
Dex because the `argo-cd-cli` client whitelist only allows `localhost`, and adding wildcard URIs
is a security risk.
