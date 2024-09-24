## Failed to parse new settings

### error converting YAML to JSON

YAML syntax is incorrect.

**incorrect:**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.slack: |
    token: $slack-token
    icon: :rocket:
```

**correct:**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.slack: |
    token: $slack-token
    icon: ":rocket:"
```

### service type 'xxxx' is not supported

You need to check your argocd-notifications controller version. For instance, the teams integration is to support `v1.1.0` and more.

## Failed to notify recipient

### notification service 'xxxx' is not supported

You have not defined `xxxx` in `argocd-notifications-cm` or to fail to parse settings.

### GitHub.repoURL (\u003cno value\u003e) does not have a / using the configuration

You probably have an Application with [multiple sources](https://argo-cd.readthedocs.io/en/stable/user-guide/multiple_sources/):

```yaml
spec:
  sources:  # <- multiple sources
  - repoURL: https://github.com/exampleOrg/first.git
    path: sources/example
  - repoURL: https://github.com/exampleOrg/second.git
    targetRevision: "{{branch}}"
```

So standard notification template won't work (`{{.app.spec.source.repoURL}}`). You should choose a single source instead:

```yaml
template.example: |
  github:
    repoURLPath: "{{ (index .app.spec.sources 0).repoURL }}"
```

### Error message `POST https://api.github.com/repos/xxxx/yyyy/statuses/: 404 Not Found`

This case is similar to the previous one, you have multiple sources in the Application manifest. 
Default `revisionPath` template `{{.app.status.operationState.syncResult.revision}}` is for an Application with single source.
Here is the link to the [source code](https://github.com/argoproj/argo-cd/blob/b8249567ae1afe657f3d2f235dc3724880c91370/pkg/apis/application/v1alpha1/types.go#L1407) 
where `revisions` field is defined.

Application status in yaml for multiple source is like this:

```yaml
status:
  operationState:
    syncResult:
      revisions:
        - 38cfa22edf9148caabfecb288bfb47dc4352dfc6
        - 38cfa22edf9148caabfecb288bfb47dc4352dfc6
```

Quick fix for this is to use `index` function to get the first revision:
```yaml
template.example: |
  github:
    revisionPath: "{{index .app.status.operationState.syncResult.revisions 0}}"
```

## config referenced xxx, but key does not exist in secret

- If you are using a custom secret, check that the secret is in the same namespace
- You have added the label: `app.kubernetes.io/part-of: argocd` to the secret
- You have tried restarting argocd-notifications controller

### Example:
Secret:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: argocd-slackbot
  namespace: <the namespace where argocd is installed>
  labels:
    app.kubernetes.io/part-of: argocd
type: Opaque
data:
  slack-token: <base64encryptedtoken>
```
ConfigMap
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.slack: |
    token: $argocd-slackbot:slack-token
```
