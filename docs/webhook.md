# Git Webhook Configuration

## Overview

Argo CD polls git repositories every three minutes to detect changes to the manifests. To eliminate
this delay from polling, the API server can be configured to receive webhook events. Argo CD supports
git webhook notifications from GitHub, GitLab, and BitBucket. The following explains how to configure
a git webhook for GitHub, but the same process should be applicable to other providers.

### 1. Create the webhook in the git provider

In your git provider, navigate to the settings page where webhooks can be configured. The payload
URL configured in the git provider should use the `/api/webhook` endpoint of your Argo CD instance
(e.g. https://argocd.example.com/api/webhook). If you wish to use a shared secret, input an
arbitrary value in the secret. This value will be used when configuring the webhook in the next step.

![Add Webhook](assets/webhook-config.png "Add Webhook")

### 2. Configure Argo CD with the webhook secret (optional)

Configuring a webhook shared secret is optional, since Argo CD will still refresh applications
related to the git repository, even with unauthenticated webhook events. This is safe to do since
the contents of webhook payloads are considered untrusted, and will only result in a refresh of the
application (a process which already occurs at three-minute intervals). If Argo CD is publicly
accessible, then configuring a webhook secret is recommended to prevent a DDoS attack.

In the `argocd-secret` kubernetes secret, configure one of the following keys with the git
provider's webhook secret configured in step 1.

| Provider  | K8s Secret Key           |
|---------- | ------------------------ |
| GitHub    | `github.webhook.secret`  |
| GitLab    | `gitlab.webhook.secret`  |
| BitBucket | `bitbucket.webhook.uuid` |

Edit the Argo CD kubernetes secret:
```
kubectl edit secret argocd-secret -n argocd
```

TIP: for ease of entering secrets, kubernetes supports inputting secrets in the `stringData` field,
which saves you the trouble of base64 encoding the values and copying it to the `data` field.
Simply copy the shared webhook secret created in step 1, to the corresponding
GitHub/GitLab/BitBucket key under the `stringData` field:

```
apiVersion: v1
kind: Secret
metadata:
  name: argocd-secret
  namespace: argocd
type: Opaque
data:
...

stringData:
  # github webhook secret
  github.webhook.secret: shhhh! it's a github secret

  # gitlab webhook secret
  gitlab.webhook.secret: shhhh! it's a gitlab secret

  # bitbucket webhook secret
  bitbucket.webhook.uuid: your-bitbucket-uuid

```

After saving, the changes should take affect automatically.
