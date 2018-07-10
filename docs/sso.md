# SSO Configuration

## Overview

ArgoCD embeds and bundles [Dex](https://github.com/coreos/dex) as part of its installation, for the
purpose of delegating authentication to an external identity provider. Multiple types of identity
providers are supported (OIDC, SAML, LDAP, GitHub, etc...). SSO configuration of ArgoCD requires
editing the `argocd-cm` ConfigMap with 
[Dex connector](https://github.com/coreos/dex/tree/master/Documentation/connectors) settings. 

This document describes how to configure ArgoCD SSO using GitHub (OAuth2) as an example, but the
steps should be similar for other identity providers.

### 1. Register the application in the identity provider

In GitHub, register a new application. The callback address should be the `/api/dex/callback`
endpoint of your ArgoCD URL (e.g. https://argocd.example.com/api/dex/callback).

![Register OAuth App](assets/register-app.png "Register OAuth App")

After registering the app, you will receive an OAuth2 client ID and secret. These values will be
inputted into the ArgoCD configmap.

![OAuth2 Client Config](assets/oauth2-config.png "OAuth2 Client Config")

### 2. Configure ArgoCD for SSO

Edit the argocd-cm configmap:
```
kubectl edit configmap argocd-cm
```

* In the `url` key, input the base URL of ArgoCD. In this example, it is https://argocd.example.com
* In the `dex.config` key, add the `github` connector to the `connectors` sub field. See Dex's
  [GitHub connector](https://github.com/coreos/dex/blob/master/Documentation/connectors/github.md)
  documentation for explanation of the fields. A minimal config should populate the clientID,
  clientSecret generated in Step 1.
* You will very likely want to restrict logins to one ore more GitHub organization. In the
  `connectors.config.orgs` list, add one or more GitHub organizations. Any member of the org will
  then be able to login to ArgoCD to perform management tasks.

```
data:
  url: https://argocd.example.com

  dex.config: |
    connectors:
      # GitHub example
      - type: github
        id: github
        name: GitHub
        config:
          clientID: aabbccddeeff00112233
          clientSecret: $dex.github.clientSecret
          orgs:
          - name: your-github-org

      # GitHub enterprise example
      - type: github
        id: acme-github
        name: Acme GitHub
        config:
          hostName: github.acme.com
          clientID: abcdefghijklmnopqrst
          clientSecret: $dex.acme.clientSecret
          orgs:
          - name: your-github-org

      # OIDC example (e.g. Okta)
      - type: oidc
        id: okta
        name: Okta
        config:
          issuer: https://dev-123456.oktapreview.com
          clientID: aaaabbbbccccddddeee
          clientSecret: $dex.okta.clientSecret
```

After saving, the changes should take affect automatically.

NOTES:
* Any values which start with '$' will look to a key in argocd-secret of the same name (minus the $),
  to obtain the actual value. This allows you to store the `clientSecret` as a kubernetes secret.
* There is no need to set `redirectURI` in the `connectors.config` as shown in the dex documentation.
  ArgoCD will automatically use the correct `redirectURI` for any OAuth2 connectors, to match the
  correct external callback URL (e.g. https://argocd.example.com/api/dex/callback)

