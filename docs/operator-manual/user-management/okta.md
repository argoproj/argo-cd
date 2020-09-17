# Okta

!!! note "Are you using this? Please contribute!"
    If you're using this IdP please consider [contributing](../../developer-guide/site.md) to this document.

A working Single Sign-On configuration using Okta via at least two methods was achieved using:

* [SAML (with Dex)](#saml-with-dex)
* [OIDC (without Dex)](#oidc-without-dex)

## SAML (with Dex)

1. Create a new SAML application in Okta UI.
    * ![Okta SAML App 1](../../assets/saml-1.png)
        I've disabled `App Visibility` because Dex doesn't support Provider-initiated login flows.
    * ![Okta SAML App 2](../../assets/saml-2.png)
1. Click `View setup instructions` after creating the application in Okta.
    * ![Okta SAML App 3](../../assets/saml-3.png)
1. Copy the SSO URL to the `argocd-cm` in the data.oicd
1. Download the CA certificate to use in the `argocd-cm` configuration.  If you are using this in the caData field, you will need to pass the entire certificate (including `-----BEGIN CERTIFICATE-----` and `-----END CERTIFICATE-----` stanzas) through base64 encoding, for example, `base64 my_cert.pem`.
    * ![Okta SAML App 4](../../assets/saml-4.png)
1. Edit the `argocd-cm` and configure the `data.dex.config` section:

<!-- markdownlint-disable MD046 -->
```yaml
dex.config: |
  logger:
    level: debug
    format: json
  connectors:
  - type: saml
    id: okta
    name: Okta
    config:
      ssoURL: https://yourorganization.oktapreview.com/app/yourorganizationsandbox_appnamesaml_2/rghdr9s6hg98s9dse/sso/saml
      # You need `caData` _OR_ `ca`, but not both.
      caData: |
        <CA cert passed through base64 encoding>
      # You need `caData` _OR_ `ca`, but not both.
      ca: /path/to/ca.pem
      redirectURI: https://ui.argocd.yourorganization.net/api/dex/callback
      usernameAttr: email
      emailAttr: email
      groupsAttr: group
```
<!-- markdownlint-enable MD046 -->

----

## OIDC (without Dex)

!!! warning "Do you want groups for RBAC later?"
    If you want `groups` scope returned from Okta you need to unfortunately contact support to enable [API Access Management with Okta](https://developer.okta.com/docs/concepts/api-access-management/) or [_just use SAML above!_](#saml-with-dex)

    Next you may need the API Access Management feature, which the support team can enable for your OktaPreview domain for testing, to enable "custom scopes" and a separate endpoint to use instead of the "public" `/oauth2/v1/authorize` API Access Management endpoint. This might be a paid feature if you want OIDC unfortunately. The free alternative I found was SAML.

1. On the `Okta Admin` page, navigate to the Okta API Management at `Security > API`.
    ![Okta API Management](../../assets/api-management.png)
1. Choose your `default` authorization server.
1. Click `Scopes > Add Scope`
    1. Add a scope called `groups`.
    ![Groups Scope](../../assets/groups-scope.png)
1. Click `Claims > Add Claim.`
    1. Add a claim called `groups`
    1. Choose the matching options you need, one example is:
        * e.g. to match groups starting with `argocd-` you'd return an `ID Token` using your scope name from step 3 (e.g. `groups`) where the groups name `matches` the `regex` `argocd-.*`
    ![Groups Claim](../../assets/groups-claim.png)
1. Edit the `argocd-cm` and configure the `data.oidc.config` section:

<!-- markdownlint-disable MD046 -->
```yaml
oidc.config: |
  name: Okta
  issuer: https://yourorganization.oktapreview.com
  clientID: 0oaltaqg3oAIf2NOa0h3
  clientSecret: ZXF_CfUc-rtwNfzFecGquzdeJ_MxM4sGc8pDT2Tg6t
  requestedScopes: ["openid", "profile", "email", "groups"]
  requestedIDTokenClaims: {"groups": {"essential": true}}
```
<!-- markdownlint-enable MD046 -->
