# OneLogin

!!! note "Are you using this? Please contribute!"
    If you're using this IdP please consider [contributing](../../developer-guide/site.md) to this document.

<!-- markdownlint-disable MD033 -->
<div style="text-align:center"><img src="../../../assets/argo.png" /></div>
<!-- markdownlint-enable MD033 -->

# Integrating OneLogin and ArgoCD

These instructions will take you through the entire process of getting your ArgoCD application authenticating with OneLogin. You will create a custom OIDC application within OneLogin and configure ArgoCD to use OneLogin for authentication, using UserRoles set in OneLogin to determine privileges in Argo.

## Creating and Configuring OneLogin App

For your ArgoCD application to communicate with OneLogin, you will first need to create and configure the OIDC application on the OneLogin side.

### Create OIDC Application

To create the application, do the following:

1. Navigate to your OneLogin portal, then Administration > Applications.
2. Click "Add App".
3. Search for "OpenID Connect" in the search field.
4. Select the "OpenId Connect (OIDC)" app to create.
5. Update the "Display Name" field (could be something like "ArgoCD (Production)".
6. Click "Save".

### Configuring OIDC Application Settings

Now that the application is created, you can configure the settings of the app.

#### Configuration Tab

Update the "Configuration" settings as follows:

1. Select the "Configuration" tab on the left.
2. Set the "Login Url" field to https://argocd.myproject.com/auth/login, replacing the hostname with your own.
3. Set the "Redirect Url" field to https://argocd.myproject.com/auth/callback, replacing the hostname with your own.
4. Click "Save".

!!! note "OneLogin may not let you save any other fields until the above fields are set."

#### Info Tab

You can update the "Display Name", "Description", "Notes", or the display images that appear in the OneLogin portal here.

#### Parameters Tab

This tab controls what information is sent to Argo in the token. By default it will contain a Groups field and "Credentials are" is set to "Configured by admin". Leave "Credentials are" as the default.

How the Value of the Groups field is configured will vary based on your needs, but to use OneLogin User roles for ArgoCD privileges, configure the Value of the Groups field with the following:

1. Click "Groups". A modal appears.
2. Set the "Default if no value selected" field to "User Roles".
3. Set the transform field (below it) to "Semicolon Delimited Input".
4. Click "Save".

When a user attempts to login to Argo with OneLogin, the User roles in OneLogin, say, Manager, ProductTeam, and TestEngineering, will be included in the Groups field in the token. These are the values needed for Argo to assign permissions.

The groups field in the token will look similar to the following:

```
"groups": [
    "Manager",
    "ProductTeam",
    "TestEngineering",
  ],
```

#### Rules Tab

To get up and running, you do not need to make modifications to any settings here.

#### SSO Tab

This tab contains much of the information needed to be placed into your ArgoCD configuration file (API endpoints, client ID, client secret).

Confirm "Application Type" is set to "Web".

Confirm "Token Endpoint" is set to "Basic".

#### Access Tab

This tab controls who can see this application in the OneLogin portal.

Select the roles you wish to have access to this application and click "Save".

#### Users Tab

This tab shows you the individual users that have access to this application (usually the ones that have roles specified in the Access Tab).

To get up and running, you do not need to make modifications to any settings here.

#### Privileges Tab

This tab shows which OneLogin users can configure this app.

To get up and running, you do not need to make modifications to any settings here.

## Updating OIDC configuration in ArgoCD

Now that the OIDC application is configured in OneLogin, you can update Argo configuration to communicate with OneLogin, as well as control permissions for those users that authenticate via OneLogin.

### Tell Argo where OneLogin is

Argo needs to have its config map (argocd-cm) updated in order to communicate with OneLogin. Consider the following yaml:

```
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
data:
  url: https://<argocd.myproject.com>
  oidc.config: |
    name: OneLogin
    issuer: https://openid-connect.onelogin.com/oidc
    clientID: aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaaaaaaaa
    # This is a base64 encoded value!
    clientSecret: YWFiYmNjZGQ=  

    # Optional set of OIDC scopes to request. If omitted, defaults to: ["openid", "profile", "email", "groups"]
    requestedScopes: ["openid", "profile", "email", "groups"]
```

The "url" key should have a value of the hostname of your Argo project.

The "clientID" is taken from the SSO tab of the OneLogin application.

The “issuer” is taken from the SSO tab of the OneLogin application. It is one of the issuer api endpoints.

The "clientSecret" value should be a base64 encoded version of the client secret located in the SSO tab of the OneLogin application. To generate this value, you can take the value of the client secret in OneLogin, and pipe it into base64 in a Linux/Unix terminal like so:

```
$ echo -n "aabbccdd" | base64
YWFiYmNjZGQ=
```

!!! note "If you get an `invalid_client` error when trying the authenticate with OneLogin, there is a possibility that your client secret was not [correctly] base64 encoded.""

### Configure Permissions for OneLogin Auth'd Users

Permissions in ArgoCD can be configured by using the OneLogin role names that are passed in the Groups field in the token. Consider the following yaml in argocd-rbac-cm.yaml:

```
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-rbac-cm
  namespace: argocd
data:
  policy.default: role:readonly
  policy.csv: |
    p, role:org-admin, applications, *, */*, allow
    p, role:org-admin, clusters, get, *, allow
    p, role:org-admin, repositories, get, *, allow
    p, role:org-admin, repositories, create, *, allow
    p, role:org-admin, repositories, update, *, allow
    p, role:org-admin, repositories, delete, *, allow

    g, TestEngineering, role:org-admin
```

In OneLogin, a user with user role "TestEngineering" will receive ArgoCD admin privileges when they log in to Argo via OneLogin. All other users will receive the readonly role. The key takeaway here is that "TestEngineering" is passed via the Group field in the token (which is specified in the Parameters tab in OneLogin).
