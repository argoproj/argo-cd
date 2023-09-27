# Identity Center (AWS SSO)

!!! note "Are you using this? Please contribute!"
    If you're using this IdP please consider [contributing](../../developer-guide/site.md) to this document.

A working Single Sign-On configuration using Identity Center (AWS SSO) has been achieved using the following method:

* [SAML (with Dex)](#saml-with-dex)

## SAML (with Dex)

1. Create a new SAML application in Identity Center and download the certificate.
    * ![Identity Center SAML App 1](../../assets/identity-center-1.png)
    * ![Identity Center SAML App 2](../../assets/identity-center-2.png)
1. Click `Assign Users` after creating the application in Identity Center and select the users or user groups you want to allow to use this application..
    * ![Identity Center SAML App 3](../../assets/identity-center-3.png)
1. Copy the Argo CD URL to the `argocd-cm` in the data.url

<!-- markdownlint-disable MD046 -->
```yaml
data:
  url: https://argocd.example.com
```
1. Configure Attribute mappings
    * ![Identity Center SAML App 4](../../assets/identity-center-4.png)
    * ![Identity Center SAML App 5](../../assets/identity-center-5.png)

<!-- markdownlint-disable MD046 -->

1. Download the CA certificate to use in the `argocd-cm` configuration.
    * If you are using this in the caData field, you will need to pass the entire certificate (including `-----BEGIN CERTIFICATE-----` and `-----END CERTIFICATE-----` stanzas) through base64 encoding, for example, `base64 my_cert.pem`.
    * If you are using the ca field and storing the CA certificate separately as a secret, you will need to mount the secret to the `dex` container in the `argocd-dex-server` Deployment.
    * ![Identity Center SAML App 6](../../assets/identity-center-6.png)
1. Edit the `argocd-cm` and configure the `data.dex.config` section:

<!-- markdownlint-disable MD046 -->
```yaml
dex.config: |
  logger:
    level: debug
    format: json
  connectors:
  - type: saml
    id: aws
    name: "AWS IAM Identity Center"
    config:
      # You need value of Identity Center APP SAML (IAM Identity Center sign-in URL)
      ssoURL: https://portal.sso.yourregion.amazonaws.com/saml/assertion/id
      # You need `caData` _OR_ `ca`, but not both.
      caData: <CA cert (IAM Identity Center Certificate of Identity Center APP SAML) passed through base64 encoding>
      # Path to mount the secret to the dex container
      entityIssuer: https://external.path.to.argocd.io/api/dex/callback
      redirectURI: https://external.path.to.argocd.io/api/dex/callback
      usernameAttr: email
      emailAttr: email
      groupsAttr: groups
```
<!-- markdownlint-enable MD046 -->

### Connect Identity Center Groups to Argo CD Roles
Argo CD is aware of user memberships of Identity Center groups that match the *Group Attribute Statements* regex.
The example above uses the `argocd-*` regex, so Argo CD would be aware of a group named `argocd-admins`.

Modify the `argocd-rbac-cm` ConfigMap to connect the `ArgoCD-administrators` Identity Center group to the builtin Argo CD `admin` role.
<!-- markdownlint-disable MD046 -->
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-rbac-cm
data:
  policy.csv: |
    g, <Identity Center Group ID>, role:admin
  scopes: '[groups, email]'
```

<!-- markdownlint-disable MD046 -->
<!-- markdownlint-enable MD046 -->
