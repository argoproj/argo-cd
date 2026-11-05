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
- PKCE (`code_challenge` / `code_challenge_method=S256`) is always included in the device
  authorization request and `code_verifier` is sent with the token request. Providers that
  enforce PKCE globally (e.g. Keycloak with **PKCE Challenge Method = S256**) require this even for
  the device flow and verify the code_verifier against the challenge. 
  Providers without PKCE support are expected to ignore the unknown parameters.

**Dex configuration (`util/dex/config.go`)**

The Dex configuration is updated as follows:
- The `argo-cd-cli` static client's `redirectURIs` now includes `/api/dex/device/callback` (required by Dex internally during the device flow).
- If the operator has explicitly configured `oauth2.grantTypes` without the device code grant, Argo CD leaves the list unchanged
and logs a warning; the operator must add the grant to enable the device flow.

### Security Considerations

- The device code flow does not use redirect URIs visible to the user, eliminating the open-redirect
  risk that motivated the `argo-cd-cli` localhost-only whitelist.
- Device codes are short-lived (`expires_in`, typically 5–10 minutes) and single-use.
- The user code displayed on screen conveys no secret; the actual authorization token is never shown.
- Polling uses the server-specified `interval` to avoid triggering rate limits; `slow_down` responses
  further increase the interval.

### Risks and Mitigations

| Risk                                             | Mitigation                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
|--------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| OIDC provider does not support device code grant | If the provider rejects the device code request, the CLI exits with `Failed to request device code: <provider error>`. If the provider issues a device code but does not allow the grant (for example, Dex with `oauth2.grantTypes` set without `urn:ietf:params:oauth:grant-type:device_code`), the CLI prints the verification URL and then exits on the first poll with `oauth2: "unsupported_grant_type"`; Argo CD logs a warning when it generates such a Dex configuration. |
| Provider requires PKCE for all flows             | PKCE (`code_challenge=S256`) is always sent with the device authorization request and `code_verifier` is always sent with the token request. This satisfies providers such as Keycloak with **PKCE Method = S256** enabled. Providers without PKCE support are expected to ignore the extra parameters.                                                                                                                                                                           |
| User cancels on the provider side but CLI hangs  | CLI exits when `expired_token` is returned or `Ctrl+C` cancels the context                                                                                                                                                                                                                                                                                                                                                                                                        |

### Upgrade / Downgrade Strategy

- Purely additive: new flag, no changes to existing behaviour.
- Dex `redirectURIs` change is backward-compatible; existing clients are unaffected.
- Downgrading: `--no-browser` flag will not exist in older CLI versions; users fall back to `--sso`.

## Alternatives

**`--callback` flag** (PR #22784): Allows overriding the redirect URI. Does not work with bundled
Dex because the `argo-cd-cli` client whitelist only allows `localhost`, and adding wildcard URIs
is a security risk.
