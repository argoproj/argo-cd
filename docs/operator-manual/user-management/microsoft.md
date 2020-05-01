# Microsoft

!!! note "Are you using this? Please contribute!"
    If you're using this IdP please consider [contributing](../../developer-guide/site.md) to this document.

* [OIDC (without Dex) to Azure AD](#oidc-without-dex-to-azure-ad)
* [With Dex](#with-dex)

## OIDC (without Dex) to Azure AD

1. Register a new Azure AD Application

    [Quickstart: Register an application](https://docs.microsoft.com/en-us/azure/active-directory/develop/quickstart-register-app)

        App Registrations Inputs
            Redirect URI: https://argocd.example.com/auth/callback
        Outputs
            Application (client) ID: aaaaaaaa-1111-bbbb-2222-cccccccccccc
            Directory (tenant) ID: 33333333-dddd-4444-eeee-555555555555
            Secret: some_secret

2. Setup permissions for Azure AD Application

    On "API permissions" page find `User.Read` permission (under `Microsoft Graph`) and grant it to the created application:

    ![Azure AD API permissions](../../assets/azure-api-permissions.png "Azure AD API permissions")

    Also, on "Token Configuration" page add groups claim for the groups assigned to the application:

    ![Azure AD token configuration](../../assets/azure-token-configuration.png "Azure AD token configuration")

3. Edit `argocd-cm` and configure the `data.oidc.config` section:

        ConfigMap -> argocd-cm
        
        data:
            url: https://argocd.example.com/
            oidc.config: |
                name: Azure
                issuer: https://login.microsoftonline.com/{directory_tenant_id}/v2.0
                clientID: {azure_ad_application_client_id}
                clientSecret: $oidc.azure.clientSecret
                requestedIDTokenClaims:
                    groups:
                        essential: true
                requestedScopes:
                    - openid
                    - profile
                    - email

4. Edit `argocd-secret` and configure the `data.oidc.azure.clientSecret` section:

        Secret -> argocd-secret
        
        data:
            oidc.azure.clientSecret: {client_secret | base64_encoded}

5. Edit `argocd-rbac-cm` to configure permissions. Use group ID from Azure for assigning roles

    [RBAC Configurations](../rbac.md)

        ConfigMap -> argocd-cm

        policy.default: role:readonly
        policy.csv: |
            p, role:org-admin, applications, *, */*, allow
            p, role:org-admin, clusters, get, *, allow
            p, role:org-admin, repositories, get, *, allow
            p, role:org-admin, repositories, create, *, allow
            p, role:org-admin, repositories, update, *, allow
            p, role:org-admin, repositories, delete, *, allow
            g, "84ce98d1-e359-4f3b-85af-985b458de3c6", role:org-admin

6. Mapping role from jwt token to argo

    If you want to map the roles from the jwt token to match the default roles (readonly and admin) then you must change the scope variable in the rbac-configmap.
        
        scopes: '[roles, email]'

## With Dex

```yaml
ConfigMap -> argocd-cm

data:
    dex.config: |
      connectors:
      - type: microsoft
        id: microsoft
        name: Your Company GmbH
        config:
          clientID: $MICROSOFT_APPLICATION_ID
          clientSecret: $MICROSOFT_CLIENT_SECRET
          redirectURI: http://localhost:8080/api/dex/callback
          tenant: ffffffff-ffff-ffff-ffff-ffffffffffff
          groups: 
            - DevOps
```
