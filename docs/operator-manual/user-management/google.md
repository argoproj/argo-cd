# Google

There are three different ways to integrate Argo CD login with your Google Workspace users. Generally the OpenID Connect (oidc) method would be the recommended way of doing this integration (and easier, as well...), but depending on your needs, you may choose a different option.

* [OpenID Connect using Dex](#openid-connect-using-dex)  
  This is the recommended login method if you don't need information about the groups the user belongs to. Google doesn't expose the `groups` claim via OIDC, so you won't be able to use Google Groups membership information for RBAC. 
* [SAML App Auth using Dex](#saml-app-auth-using-dex)  
  Dex [recommends avoiding this method](https://dexidp.io/docs/connectors/saml/#warning). Also, you won't get Google Groups membership information through this method.
* [OpenID Connect plus Google Groups using Dex](#openid-connect-plus-google-groups-using-dex)  
  This is the recommended method if you need to user Google Groups membership in your RBAC configuration.

Once you've set up one of the above integrations, be sure to edit `argo-rbac-cm` to configure permissions (as in the example below). See [RBAC Configurations](../rbac.md) for more detailed scenarios.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-rbac-cm
  namespace: argocd
data:
  policy.default: role:readonly
```

## OpenID Connect using Dex

### Configure your OAuth consent screen

If you've never configured this, you'll be redirected straight to this if you try to create an OAuth Client ID

1. Go to your [OAuth Consent](https://console.cloud.google.com/apis/credentials/consent) configuration. If you still haven't created one, select `Internal` or `External` and click `Create` 
2. Go and [edit your OAuth consent screen](https://console.cloud.google.com/apis/credentials/consent/edit) Verify you're in the correct project!
3. At least, configure a name for your login app and a user support email address
4. The app logo and filling the information links it's not mandatory, but it's a nice touch for the login page
5. In "Authorized domains" add the domains who are allowed to log in to ArgoCD (e.g. if you add `example.com`, all Google Workspace users with an `@example.com` address can log in)
6. Save to continue to the "Scopes" section
7. Add the `.../auth/userinfo.profile` and the `openid` scopes
8. Save, review the summary of your changes and finish

### Configure a new OAuth Client ID

1. Go to your [Google API Credentials](https://console.cloud.google.com/apis/credentials) console, and make sure you're in the correct project.
2. Click on "+Create Credentials"/"OAuth Client ID"
3. Select "Web Application" in the Aplication Type drop down menu, and enter an identifying name to your app (e.g. `Argo CD`)
4. Fill "Authorized JavaScript origins" with your Argo CD URL, e.g. `https://argocd.example.com`
5. Fill "Authorized redirect URIs" with your Argo CD URL plus `/api/dex/callback`, e.g. `https://argocd.example.com/api/dex/callback`

    ![](../../assets/google-admin-oidc-uris.png)

6. Click "Create" and save your "Client ID" and your "Client Secret" for later

### Configure Argo to use OpenID Connect

Edit `argo-cm` and add the following dex.config to the data section, replacing `clientID` and `clientSecret` with the values you saved before:

```yaml
data:
  url: https://argocd.example.com
  dex.config: |
    connectors:
    - config:
        issuer: https://accounts.google.com
        clientID: XXXXXXXXXXXXX.apps.googleusercontent.com
        clientSecret: XXXXXXXXXXXXX
      type: oidc
      id: google
      name: Google
```

### References

- [Dex oidc connector docs](https://dexidp.io/docs/connectors/oidc/)

## SAML App Auth using Dex

### Configure a new SAML App

---
!!! warning "Deprecation Warning"

    Note that, according to [Dex documentation](https://dexidp.io/docs/connectors/saml/#warning), SAML is considered unsafe and they are planning to deprecate that module.

---

1. In the [Google admin console](https://admin.google.com), open the left-side menu and select `Apps` > `SAML Apps`

    ![Google Admin Apps Menu](../../assets/google-admin-saml-apps-menu.png "Google Admin menu with the Apps / SAML Apps path selected")

2. Under `Add App` select `Add custom SAML app`

    ![Google Admin Add Custom SAML App](../../assets/google-admin-saml-add-app-menu.png "Add apps menu with add custom SAML app highlighted")

3. Enter a `Name` for the application (e.g. `Argo CD`), then choose `Continue`

    ![Google Admin Apps Menu](../../assets/google-admin-saml-app-details.png "Add apps menu with add custom SAML app highlighted")

4. Download the metadata or copy the `SSO URL`, `Certificate`, and optionally `Entity ID` from the identity provider details for use in the next section. Choose `continue`.
    - Base64 encode the contents of the certificate file, for example:
    - `$ cat ArgoCD.cer | base64`
    - *Keep a copy of the encoded output to be used in the next section.*
    - *Ensure that the certificate is in PEM format before base64 encoding*

    ![Google Admin IdP Metadata](../../assets/google-admin-idp-metadata.png "A screenshot of the Google IdP metadata")

5. For both the `ACS URL` and `Entity ID`, use your Argo Dex Callback URL, for example: `https://argocd.example.com/api/dex/callback`

    ![Google Admin Service Provider Details](../../assets/google-admin-service-provider-details.png "A screenshot of the Google Service Provider Details")

6. Add SAML Attribute Mapping, Map `Primary email` to `name` and `Primary Email` to `email`. and click `ADD MAPPING` button.

    ![Google Admin SAML Attribute Mapping Details](../../assets/google-admin-saml-attribute-mapping-details.png "A screenshot of the Google Admin SAML Attribute Mapping Details")

7. Finish creating the application.

### Configure Argo to use the new Google SAML App

Edit `argo-cm` and add the following `dex.config` to the data section, replacing the `caData`, `argocd.example.com`, `sso-url`, and optionally `google-entity-id` with your values from the Google SAML App:

```yaml
data:
  url: https://argocd.example.com
  dex.config: |
    connectors:
    - type: saml
      id: saml
      name: saml
      config:
        ssoURL: https://sso-url (e.g. https://accounts.google.com/o/saml2/idp?idpid=Abcde0)
        entityIssuer: https://argocd.example.com/api/dex/callback
        caData: |
          BASE64-ENCODED-CERTIFICATE-DATA
        redirectURI: https://argocd.example.com/api/dex/callback
        usernameAttr: name
        emailAttr: email
        # optional
        ssoIssuer: https://google-entity-id (e.g. https://accounts.google.com/o/saml2?idpid=Abcde0)
```

### References

- [Dex SAML connector docs](https://dexidp.io/docs/connectors/saml/)
- [Google's SAML error messages](https://support.google.com/a/answer/6301076?hl=en)

## OpenID Connect plus Google Groups using Dex

 TODO