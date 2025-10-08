# OIDC Distributed Claims Support

Argo CD supports OIDC distributed claims as defined in the [OpenID Connect Core 1.0 specification](https://openid.net/specs/openid-connect-core-1_0.html#AggregatedDistributedClaims). This feature is particularly useful for Azure AD environments where users belong to more than 200 groups.

## Background

Some OIDC providers, notably Azure AD, use distributed claims when the number of claims exceeds certain limits. For Azure AD, when a user is a member of more than 200 groups, the provider switches from including all group memberships directly in the JWT to using distributed claims. This means group information is provided via a separate endpoint that must be queried by the OIDC client.

Without distributed claims support, users with many group memberships would appear to have no groups in Argo CD, effectively breaking RBAC authorization.

## Configuration

To enable distributed claims support in Argo CD, add the following configuration to your OIDC settings:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  oidc.config: |
    name: Azure AD
    issuer: https://login.microsoftonline.com/{tenant-id}/v2.0
    clientID: {client-id}
    clientSecret: {client-secret}
    requestedScopes: 
      - openid
      - profile
      - email
      - groups
    enableDistributedClaims: true
    distributedClaimsTimeout: "10s"
```

### Configuration Options

- **`enableDistributedClaims`** (boolean): Enable distributed claims fetching. Default: `false`
- **`distributedClaimsTimeout`** (duration): Timeout for HTTP requests to distributed claims endpoints. Default: `10s`

## How It Works

1. **Detection**: When Argo CD receives a JWT token, it checks for the presence of `_claim_names` and `_claim_sources` fields that indicate distributed claims.

2. **Fetching**: If distributed claims are detected and enabled, Argo CD makes HTTP requests to the endpoints specified in `_claim_sources` to fetch the additional claims:
   - **Azure AD**: Uses POST requests to `/me/GetMemberGroups` with JSON body `{"securityEnabledOnly": false}`
   - **Other providers**: Uses standard GET requests

3. **Merging**: The fetched claims (such as group memberships) are merged into the original JWT claims for use in RBAC decisions.

4. **Fallback**: If distributed claims fetching fails for any reason, Argo CD gracefully falls back to using the original JWT claims without the distributed information.

## Azure AD Example

Here's a complete example for configuring Azure AD with distributed claims support:

### 1. Configure Azure AD Application

In your Azure AD application registration:
- Add the following API permissions: `User.Read`, `GroupMember.Read.All` 
- Grant admin consent for these permissions
- Ensure "Groups" is included in the ID token claims (this will be omitted when users have 200+ groups, triggering distributed claims)
- Configure the redirect URI to point to your Argo CD instance (e.g., `https://argocd.example.com/auth/callback`)

### 2. Configure Argo CD

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  url: https://argocd.example.com
  oidc.config: |
    name: Azure AD
    issuer: https://login.microsoftonline.com/{your-tenant-id}/v2.0
    clientID: {your-client-id}
    clientSecret: {your-client-secret}
    requestedScopes:
      - openid
      - profile
      - email
      - groups
    enableDistributedClaims: true
    distributedClaimsTimeout: "15s"
```

### 3. Configure RBAC

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-rbac-cm
  namespace: argocd
data:
  policy.default: role:readonly
  policy.csv: |
    # Azure AD group-based policies
    g, {azure-ad-group-id}, role:admin
    g, {another-group-id}, role:readonly
```

## Troubleshooting

### Enable Debug Logging

To debug distributed claims issues, enable debug logging:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cmd-params-cm
  namespace: argocd
data:
  server.log.level: debug
```

Look for log messages containing "distributed claims" to see the fetching process.

### Common Issues

1. **Access Token Missing**: Distributed claims endpoints typically require access tokens. Ensure your OIDC provider includes access tokens in the authentication flow.

2. **Network Connectivity**: Argo CD must be able to reach the distributed claims endpoints. Check network policies and firewall rules.

3. **Timeout Issues**: If claims endpoints are slow, increase the `distributedClaimsTimeout` value.

4. **Subject Mismatch**: Distributed claims responses must have the same subject (`sub`) claim as the original JWT. Subject mismatches are rejected for security.

### Monitoring

Monitor the following metrics to track distributed claims usage:
- Check Argo CD server logs for distributed claims processing
- Monitor HTTP request failures to claims endpoints
- Verify that users with many groups can still access resources

## Security Considerations

- Distributed claims endpoints are accessed using access tokens from the original OAuth2 flow
- All distributed claims responses are validated to ensure the subject matches the original JWT
- JWT responses from distributed claims endpoints are verified using the same OIDC provider settings
- Failed distributed claims requests fall back gracefully to original JWT claims
- Timeouts prevent hanging requests from blocking authentication

## Azure AD Specific Implementation

Argo CD automatically detects Azure AD distributed claims endpoints and uses the correct Microsoft Graph API format:

- **HTTP Method**: POST (instead of GET for other providers)
- **Endpoint**: Typically `https://graph.microsoft.com/v1.0/me/GetMemberGroups`  
- **Request Body**: `{"securityEnabledOnly": false}`
- **Response Format**: `{"value": ["group-id-1", "group-id-2", ...]}`

This implementation follows the [Microsoft Graph API specification](https://learn.microsoft.com/en-us/graph/api/directoryobject-getmembergroups) for retrieving user group memberships.

## Limitations

- Currently only supports HTTP(S) endpoints for distributed claims
- Does not support aggregated claims (claims that are JWE-encrypted)
- Requires the OIDC provider to include access tokens in the authentication response
- Azure AD returns group object IDs, not display names (use these IDs in RBAC policies)