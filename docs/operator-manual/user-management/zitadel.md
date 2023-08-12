# Zitadel
Please also consult the [Zitadel Documentation](https://zitadel.com/docs).
## Integrating Zitadel and ArgoCD
These instructions will take you through the entire process of getting your ArgoCD application authenticating with Zitadel. You will create an application within Zitadel and configure ArgoCD to use Zitadel for authentication using roles set in Zitadel to determine privileges in ArgoCD.

The following steps are required to integrate ArgoCD with Zitadel:
1. Create a new project and a new application in Zitadel
2. Configure the application in Zitadel
3. Set up roles in Zitadel
4. Set up a action in Zitadel
5. Configure ArgoCD configmaps
6. Test the setup

The following values will be used in this example:
- Zitadel FQDN: `auth.example.com`
- Zitadel Project: `argocd-project`
- Zitadel Application: `argocd-application`
- Zitadel Action: `groupsClaim`
- ArgoCD FQDN: `argocd.example.com`
- ArgoCD Administrator Role: `argocd_administrators`
- ArgoCD User Role: `argocd_users`

You may choose different values in your setup, these are used to keep the guide consistent.

## Setting up your project and application in Zitadel
First we will create a new project within Zitadel. Go to **Projects** and select **Create New Project**.  
You should now see the following screen.  

![image](https://github.com/argoproj/argo-cd/assets/5135902/6d4c877b-875a-4a70-9076-9305bae353be)

Check the following options:
- Assert Roles on Authentication
- Check authorization on Authentication

![image](https://github.com/argoproj/argo-cd/assets/5135902/29511997-6434-4627-8094-f91da9675072)

### Roles

Go to **Roles** and click **New**. Create the following two roles. Use the specified values below for both of the fields **Key** and **Group**.
- `argocd_administrators`
- `argocd_users`

Your roles should now look like this:

![image](https://github.com/argoproj/argo-cd/assets/5135902/54a11922-437b-4b0a-b998-5200842f2318)

### Authorizations

Next, go to **Authorizations** and assign your user the role `argocd_administrators`.
Click **New**, enter the name of your user and click **Continue**. Select the role `argocd_administrators` and click **Save**.

Your authorizations should now look like this:

![image](https://github.com/argoproj/argo-cd/assets/5135902/f3aa7f2c-29c4-4942-8e05-d736b843c043)

### Creating an application

Go to **General** and create a new application. Name the application `argocd-application`.

As type of the application, select **WEB** and click continue.

![image](https://github.com/argoproj/argo-cd/assets/5135902/36155051-47ce-4b81-8d67-0d659e0b88e9)

Select **CODE** and continue.

![image](https://github.com/argoproj/argo-cd/assets/5135902/5247118b-f5c4-429b-97ff-3755ba65513f)

Next we will set up the redirect and post-logout URIs. Set the following values:
- Redirect URI: `https://argocd.example.com/auth/callback`
- Post Logout URI: `https://argocd.example.com`

The post logout uri is optional. In the example setup users will be taken back to the ArgoCD login page after logging out.

![image](https://github.com/argoproj/argo-cd/assets/5135902/b8ee4905-967f-42ba-a205-4ff0c4c78771)

Verify your configuration on the next screen and click **Create** to create the application.

![image](https://github.com/argoproj/argo-cd/assets/5135902/30bc48c9-f2ba-4714-a3d4-6ab9210816a1)

After clicking **Create** you will be shown the `ClientId` and the `ClientSecret` for your application. Make sure to copy the ClientSecret as you will not be able to retrieve it after closing this window.  
For our example the following values are used:
- ClientId: `227060711795262483@argocd-project`
- ClientSecret: `UGvTjXVFAQ8EkMv2x4GbPcrEwrJGWZ0sR2KbwHRNfYxeLsDurCiVEpa5bkgW0pl0`

![image](https://github.com/argoproj/argo-cd/assets/5135902/30c0aa59-ae42-4f4a-ba23-3be61eaff169)

Once you have saved the ClientSecret in a safe place, click **Close** to complete creating the application.

Go to **Token Settings** and enable the following options:  
- User roles inside ID Token
- User Info inside ID Token

![image](https://github.com/argoproj/argo-cd/assets/5135902/dbd2add9-bc23-4fd8-a6a9-8784959bde0f)

## Setting up an action in Zitadel

To include the role of the user in the token issued by Zitadel, we will need to setup a Zitadel Action. The authorization in ArgoCD will be determined by the role contained within the auth token.  
Go to **Actions**, click **New** and choose `groupsClaim` as the name of your action.

Paste the following code into the action:

```javascript
/**
 * sets the roles an additional claim in the token with roles as value an project as key
 *
 * The role claims of the token look like the following:
 *
 * // added by the code below
 * "groups": ["{roleName}", "{roleName}", ...],
 *
 * Flow: Complement token, Triggers: Pre Userinfo creation, Pre access token creation
 *
 * @param ctx
 * @param api
 */
function groupsClaim(ctx, api) {
  if (ctx.v1.user.grants === undefined || ctx.v1.user.grants.count == 0) {
    return;
  }

  let grants = [];
  ctx.v1.user.grants.grants.forEach((claim) => {
    claim.roles.forEach((role) => {
      grants.push(role);
    });
  });

  api.v1.claims.setClaim("groups", grants);
}
```

Check **Allowed To Fail** and click **Add** to add your action.  

*Note: If **Allowed To Fail** is not checked and a user does not have a role assigned, it may be possible that the user is no longer able to log in to Zitadel as the login flow fails when the action fails.*

Next, add your action to the **Complement Token** flow. Select the **Complement Token** flow from the dropdown and click **Add trigger**.  
Add your action to both triggers **Pre Userinfo creation** and **Pre access token creation**.

Your Actions page should now look like the following screenshot:

![image](https://github.com/argoproj/argo-cd/assets/5135902/9a234146-d655-402e-b435-33d7aa5499ab)


## Configuring the ArgoCD configmaps

Next, we will configure two ArgoCD configmaps:
- [argocd-cm.yaml](https://github.com/argoproj/argo-cd/blob/master/docs/operator-manual/argocd-cm.yaml)
- [argocd-rbac-cm.yaml](https://github.com/argoproj/argo-cd/blob/master/docs/operator-manual/argocd-rbac-cm.yaml)

Configure your configmaps as follows:

### argocd-cm.yaml
```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
  labels:
    app.kubernetes.io/part-of: argocd
data:
  admin.enabled: "false"
  url: https://argocd.example.com
  oidc.config: |
    name: Zitadel
    issuer: https://auth.example.com
    clientID: <clientid>
    clientSecret: <clientsecret>
    requestedScopes:
      - openid
      - profile
      - email
      - groups
    logoutURL: https://auth.example.com/oidc/v1/end_session
```

### argocd-rbac-cm.yaml
```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-rbac-cm
  namespace: argocd
  labels:
    app.kubernetes.io/part-of: argocd
data:
  scopes: '[groups]'
  policy.csv: |
    g, argocd_administrators, role:admin
    g, argocd_users, role:readonly
  policy.default: ''
```

The roles specified under `policy.csv` must match the roles configured in Zitadel.  
The Zitadel role `argocd_administrators` will be assigned the ArgoCD role `admin` granting admin access to ArgoCD.  
The Zitadel role `argocd_users` will be assigned the ArgoCD role `readonly` granting read-only access to ArgoCD.

Deploy your ArgoCD configmaps. ArgoCD and Zitadel should now be setup correctly to allow users to log in to ArgoCD using Zitadel.

## Testing the setup

Go to your ArgoCD instance. You should now see the **LOG IN WITH ZITADEL** button above the usual username/password login.

![image](https://github.com/argoproj/argo-cd/assets/5135902/aed9c240-a8e6-4ca2-8609-8a713d78a911)

After loggin in with your Zitadel user go to **User Info**. If everythin is set up correctly you should now see the group `argocd_administrators` as shown below.

![image](https://github.com/argoproj/argo-cd/assets/5135902/589a015a-0628-4a60-8086-0a7e5f8202c4)
