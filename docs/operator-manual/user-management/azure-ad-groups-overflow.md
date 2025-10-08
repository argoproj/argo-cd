# Azure AD Groups Overflow Support

Argo CD supports Azure AD groups overflow handling, which occurs when users belong to more than 200 groups. In this scenario, Azure AD switches from including all group memberships directly in the JWT to using distributed claims, where group information is provided via a separate Microsoft Graph API endpoint.

## Background

Azure AD has a limitation where when a user is a member of more than 200 groups, the provider switches from including all group memberships directly in the JWT to using distributed claims. This means group information must be fetched from a separate Microsoft Graph API endpoint.

Without Azure AD groups overflow support, users with many group memberships would appear to have no groups in Argo CD, effectively breaking RBAC authorization.

## Configuration

To enable Azure AD groups overflow support in Argo CD, add the following configuration to your OIDC settings:

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
    enableAzureGroupsOverflow: true
    azureGroupsOverflowTimeout: "10s"
```

### Configuration Options

- **`enableAzureGroupsOverflow`** (boolean): Enable Azure AD groups overflow handling. Default: `false`
- **`azureGroupsOverflowTimeout`** (duration): Timeout for HTTP requests to Azure Graph API endpoints. Default: `10s`

## How It Works

1. **Detection**: When Argo CD receives a JWT token, it checks for the presence of `_claim_names` and `_claim_sources` fields with a "groups" claim that indicates Azure AD groups overflow.

2. **Fetching**: If Azure AD groups overflow is detected and enabled, Argo CD makes a POST request to the Microsoft Graph API endpoint specified in `_claim_sources` to fetch the user's group memberships using the JSON body `{"securityEnabledOnly": false}`.

3. **Merging**: The fetched group memberships are merged into the original JWT claims for use in RBAC decisions.

4. **Fallback**: If Azure AD groups overflow fetching fails for any reason, Argo CD gracefully falls back to using the original JWT claims without the overflow group information.

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
    enableAzureGroupsOverflow: true
    azureGroupsOverflowTimeout: "15s"
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

To debug Azure AD groups overflow issues, enable debug logging:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cmd-params-cm
  namespace: argocd
data:
  server.log.level: debug
```

Look for log messages containing "Azure AD groups overflow" or "Azure Graph API" to see the fetching process.

### Common Issues

1. **Access Token Missing**: Azure Graph API endpoints require access tokens. Ensure your Azure AD application includes access tokens in the authentication flow.

2. **Network Connectivity**: Argo CD must be able to reach the Microsoft Graph API endpoints. Check network policies and firewall rules.

3. **Timeout Issues**: If Graph API endpoints are slow, increase the `azureGroupsOverflowTimeout` value.

4. **Insufficient Permissions**: Ensure the Azure AD application has the necessary permissions to read group memberships (e.g., `GroupMember.Read.All`).

### Monitoring

Monitor the following metrics to track Azure AD groups overflow usage:
- Check Argo CD server logs for Azure AD groups overflow processing
- Monitor HTTP request failures to Microsoft Graph API endpoints
- Verify that users with many groups can still access resources

## Security Considerations

- Azure Graph API endpoints are accessed using access tokens from the original OAuth2 flow
- Failed Azure AD groups overflow requests fall back gracefully to original JWT claims
- Timeouts prevent hanging requests from blocking authentication
- Only group memberships are fetched, no other user information
- Access tokens are only used to call Microsoft Graph API endpoints

## Implementation Details

Argo CD automatically detects Azure AD groups overflow and uses the correct Microsoft Graph API format:

- **HTTP Method**: POST
- **Endpoint**: Converts legacy `graph.windows.net` URLs to `graph.microsoft.com/v1.0` format automatically
- **Request Body**: `{"securityEnabledOnly": false}`
- **Response Format**: `{"value": ["group-id-1", "group-id-2", ...]}`

This implementation follows the [Microsoft Graph API specification](https://learn.microsoft.com/en-us/graph/api/directoryobject-getmembergroups) for retrieving user group memberships.

## Limitations

- Only supports Azure AD groups overflow (not generic OIDC distributed claims)
- Requires the Azure AD application to include access tokens in the authentication response
- Azure AD returns group object IDs, not display names (use these IDs in RBAC policies)
- Does not support other OIDC providers' distributed claims implementations