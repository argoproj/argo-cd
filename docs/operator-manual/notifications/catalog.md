# Triggers and Templates Catalog
## Getting Started
* Install Triggers and Templates from the catalog
  ```bash
  kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/notifications_catalog/install.yaml
  ```
## Triggers
|          NAME          |                          DESCRIPTION                          |                      TEMPLATE                       |
|------------------------|---------------------------------------------------------------|-----------------------------------------------------|
| on-created             | Application is created.                                       | [app-created](#app-created)                         |
| on-deleted             | Application is deleted.                                       | [app-deleted](#app-deleted)                         |
| on-deployed            | Application is synced and healthy. Triggered once per commit. | [app-deployed](#app-deployed)                       |
| on-health-degraded     | Application has degraded                                      | [app-health-degraded](#app-health-degraded)         |
| on-sync-failed         | Application syncing has failed                                | [app-sync-failed](#app-sync-failed)                 |
| on-sync-running        | Application is being synced                                   | [app-sync-running](#app-sync-running)               |
| on-sync-status-unknown | Application status is 'Unknown'                               | [app-sync-status-unknown](#app-sync-status-unknown) |
| on-sync-succeeded      | Application syncing has succeeded                             | [app-sync-succeeded](#app-sync-succeeded)           |

## Templates
### app-created
**definition**:
```yaml
email:
  subject: Application {{.app.metadata.name}} has been created.
message: Application {{.app.metadata.name}} has been created.
teams:
  title: Application {{.app.metadata.name}} has been created.

```
### app-deleted
**definition**:
```yaml
email:
  subject: Application {{.app.metadata.name}} has been deleted.
message: Application {{.app.metadata.name}} has been deleted.
teams:
  title: Application {{.app.metadata.name}} has been deleted.

```
### app-deployed
**definition**:
```yaml
email:
  subject: New version of an application {{.app.metadata.name}} is up and running.
message: |
  {{if eq .serviceType "slack"}}:white_check_mark:{{end}} Application {{.app.metadata.name}} is now running new version of deployments manifests.
slack:
  attachments: "[{\n  \"title\": \"{{ .app.metadata.name}}\",\n  \"title_link\":\"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}\",\n
    \ \"color\": \"#18be52\",\n  \"fields\": [\n  {\n    \"title\": \"Sync Status\",\n
    \   \"value\": \"{{.app.status.sync.status}}\",\n    \"short\": true\n  },\n  {\n
    \   \"title\": \"Repository\",\n    \"value\": \n      {{- if .app.spec.source
    }}\n        \"{{ .app.spec.source.repoURL }}\"\n      {{- else if .app.spec.sources
    }}\n        {{- range .app.spec.sources }}\n          \"{{ .repoURL }}\"\n        {{-
    end }}\n      {{- else }}\n        \"no repoURL\"\n      {{- end }}\n    \"short\":
    true\n  },\n  {\n    \"title\": \"Revision\",\n    \"value\": \"{{.app.status.sync.revision}}\",\n
    \   \"short\": true\n  }\n  {{range $index, $c := .app.status.conditions}}\n  ,\n
    \ {\n    \"title\": \"{{$c.type}}\",\n    \"value\": \"{{$c.message}}\",\n    \"short\":
    true\n  }\n  {{end}}\n  ]\n}]\n"
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: "[{\n  \"name\": \"Sync Status\",\n  \"value\": \"{{.app.status.sync.status}}\"\n},\n{\n
    \ \"name\": \"Repository\",\n  \"value\": \n    {{- if .app.spec.source }}\n      \"{{
    .app.spec.source.repoURL }}\"\n    {{- else if .app.spec.sources }}\n      {{-
    range .app.spec.sources }}\n        \"{{ .repoURL }}\"\n      {{- end }}\n    {{-
    else }}\n      \"no repoURL\"\n    {{- end }}\n},\n{\n  \"name\": \"Revision\",\n
    \ \"value\": \"{{.app.status.sync.revision}}\"\n}\n{{range $index, $c := .app.status.conditions}}\n
    \ ,\n  {\n    \"name\": \"{{$c.type}}\",\n    \"value\": \"{{$c.message}}\"\n
    \ }\n{{end}}\n]\n"
  potentialAction: "[{\n  \"@type\":\"OpenUri\",\n  \"name\":\"Operation Application\",\n
    \ \"targets\":[{\n    \"os\":\"default\",\n    \"uri\":\"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}\"\n
    \ }]\n},\n{\n  \"@type\":\"OpenUri\",\n  \"name\":\"Open Repository\",\n  \"targets\":[{\n
    \   \"os\":\"default\",\n    \"uri\": \n      {{- if .app.spec.source }}\n        \"{{
    .app.spec.source.repoURL }}\"\n      {{- else if .app.spec.sources }}\n        {{-
    range .app.spec.sources }}\n          \"{{ .repoURL }}\"\n        {{- end }}\n
    \     {{- else }}\n        \"no repoURL\"\n      {{- end }}\n  }]\n}]"
  themeColor: '#000080'
  title: New version of an application {{.app.metadata.name}} is up and running.

```
### app-health-degraded
**definition**:
```yaml
email:
  subject: Application {{.app.metadata.name}} has degraded.
message: |
  {{if eq .serviceType "slack"}}:exclamation:{{end}} Application {{.app.metadata.name}} has degraded.
  Application details: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}.
slack:
  attachments: "[{\n  \"title\": \"{{ .app.metadata.name}}\",\n  \"title_link\": \"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}\",\n
    \ \"color\": \"#f4c030\",\n  \"fields\": [\n  {\n    \"title\": \"Health Status\",\n
    \   \"value\": \"{{.app.status.health.status}}\",\n    \"short\": true\n  },\n
    \ {\n    \"title\": \"Repository\",\n    \"value\": \n      {{- if .app.spec.source
    }}\n        \"{{ .app.spec.source.repoURL }}\"\n      {{- else if .app.spec.sources
    }}\n        {{- range .app.spec.sources }}\n          \"{{ .repoURL }}\"\n        {{-
    end }}\n      {{- else }}\n        \"no repoURL\"\n      {{- end }}\n    \"short\":
    true\n  }\n  {{range $index, $c := .app.status.conditions}}\n  ,\n  {\n    \"title\":
    \"{{$c.type}}\",\n    \"value\": \"{{$c.message}}\",\n    \"short\": true\n  }\n
    \ {{end}}\n  ]\n}]\n"
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: "[{\n  \"name\": \"Health Status\",\n  \"value\": \"{{.app.status.health.status}}\"\n},\n{\n
    \ \"name\": \"Repository\",\n  \"value\": \n    {{- if .app.spec.source }}\n      \"{{
    .app.spec.source.repoURL }}\"\n    {{- else if .app.spec.sources }}\n      {{-
    range .app.spec.sources }}\n        \"{{ .repoURL }}\"\n      {{- end }}\n    {{-
    else }}\n      \"no repoURL\"\n    {{- end }}\n}\n{{range $index, $c := .app.status.conditions}}\n
    \ ,\n  {\n    \"name\": \"{{$c.type}}\",\n    \"value\": \"{{$c.message}}\"\n
    \ }\n{{end}}\n]\n"
  potentialAction: "[{\n  \"@type\":\"OpenUri\",\n  \"name\":\"Open Application\",\n
    \ \"targets\":[{\n    \"os\":\"default\",\n    \"uri\":\"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}\"\n
    \ }]\n},\n{\n  \"@type\":\"OpenUri\",\n  \"name\":\"Open Repository\",\n  \"targets\":[{\n
    \   \"os\":\"default\",\n    \"uri\": \n      {{- if .app.spec.source }}\n        \"{{
    .app.spec.source.repoURL }}\"\n      {{- else if .app.spec.sources }}\n        {{-
    range .app.spec.sources }}\n          \"{{ .repoURL }}\"\n        {{- end }}\n
    \     {{- else }}\n        \"no repoURL\"\n      {{- end }}\n  }]\n}]\n"
  themeColor: '#FF0000'
  title: Application {{.app.metadata.name}} has degraded.

```
### app-sync-failed
**definition**:
```yaml
email:
  subject: Failed to sync application {{.app.metadata.name}}.
message: |
  {{if eq .serviceType "slack"}}:exclamation:{{end}}  The sync operation of application {{.app.metadata.name}} has failed at {{.app.status.operationState.finishedAt}} with the following error: {{.app.status.operationState.message}}
  Sync operation details are available at: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true .
slack:
  attachments: "[{\n  \"title\": \"{{ .app.metadata.name}}\",\n  \"title_link\":\"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}\",\n
    \ \"color\": \"#E96D76\",\n  \"fields\": [\n  {\n    \"title\": \"Sync Status\",\n
    \   \"value\": \"{{.app.status.sync.status}}\",\n    \"short\": true\n  },\n  {\n
    \   \"title\": \"Repository\",\n    \"value\": \n      {{- if .app.spec.source
    }}\n        \"{{ .app.spec.source.repoURL }}\"\n      {{- else if .app.spec.sources
    }}\n        {{- range .app.spec.sources }}\n          \"{{ .repoURL }}\"\n        {{-
    end }}\n      {{- else }}\n        \"no repoURL\"\n      {{- end }}\n    \"short\":
    true\n  }\n  {{range $index, $c := .app.status.conditions}}\n  ,\n  {\n    \"title\":
    \"{{$c.type}}\",\n    \"value\": \"{{$c.message}}\",\n    \"short\": true\n  }\n
    \ {{end}}\n  ]\n}]\n"
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: "[{\n  \"name\": \"Sync Status\",\n  \"value\": \"{{.app.status.sync.status}}\"\n},\n{\n
    \ \"name\": \"Failed at\",\n  \"value\": \"{{.app.status.operationState.finishedAt}}\"\n},\n{\n
    \ \"name\": \"Repository\",\n  \"value\": \n    {{- if .app.spec.source }}\n      \"{{
    .app.spec.source.repoURL }}\"\n    {{- else if .app.spec.sources }}\n      {{-
    range .app.spec.sources }}\n        \"{{ .repoURL }}\"\n      {{- end }}\n    {{-
    else }}\n      \"no repoURL\"\n    {{- end }}\n}\n{{range $index, $c := .app.status.conditions}}\n
    \ ,\n  {\n    \"name\": \"{{$c.type}}\",\n    \"value\": \"{{$c.message}}\"\n
    \ }\n{{end}}\n]\n"
  potentialAction: "[{\n  \"@type\":\"OpenUri\",\n  \"name\":\"Open Operation\",\n
    \ \"targets\":[{\n    \"os\":\"default\",\n    \"uri\":\"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true\"\n
    \ }]\n},\n{\n  \"@type\":\"OpenUri\",\n  \"name\":\"Open Repository\",\n  \"targets\":[{\n
    \   \"os\":\"default\",\n    \"uri\": \n      {{- if .app.spec.source }}\n        \"{{
    .app.spec.source.repoURL }}\"\n      {{- else if .app.spec.sources }}\n        {{-
    range .app.spec.sources }}\n          \"{{ .repoURL }}\"\n        {{- end }}\n
    \     {{- else }}\n        \"no repoURL\"\n      {{- end }} \n  }]\n}]"
  themeColor: '#FF0000'
  title: Failed to sync application {{.app.metadata.name}}.

```
### app-sync-running
**definition**:
```yaml
email:
  subject: Start syncing application {{.app.metadata.name}}.
message: |
  The sync operation of application {{.app.metadata.name}} has started at {{.app.status.operationState.startedAt}}.
  Sync operation details are available at: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true .
slack:
  attachments: "[{\n  \"title\": \"{{ .app.metadata.name}}\",\n  \"title_link\":\"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}\",\n
    \ \"color\": \"#0DADEA\",\n  \"fields\": [\n  {\n    \"title\": \"Sync Status\",\n
    \   \"value\": \"{{.app.status.sync.status}}\",\n    \"short\": true\n  },\n  {\n
    \   \"title\": \"Repository\",\n    \"value\": \n      {{- if .app.spec.source
    }}\n        \"{{ .app.spec.source.repoURL }}\"\n      {{- else if .app.spec.sources
    }}\n        {{- range .app.spec.sources }}\n          \"{{ .repoURL }}\"\n        {{-
    end }}\n      {{- else }}\n        \"no repoURL\"\n      {{- end }}\n    \"short\":
    true\n  }\n  {{range $index, $c := .app.status.conditions}}\n  ,\n  {\n    \"title\":
    \"{{$c.type}}\",\n    \"value\": \"{{$c.message}}\",\n    \"short\": true\n  }\n
    \ {{end}}\n  ]\n}]\n"
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: "[{\n  \"name\": \"Sync Status\",\n  \"value\": \"{{.app.status.sync.status}}\"\n},\n{\n
    \ \"name\": \"Started at\",\n  \"value\": \"{{.app.status.operationState.startedAt}}\"\n},\n{\n
    \ \"name\": \"Repository\",\n  \"value\": \n    {{- if .app.spec.source }}\n      \"{{
    .app.spec.source.repoURL }}\"\n    {{- else if .app.spec.sources }}\n      {{-
    range .app.spec.sources }}\n        \"{{ .repoURL }}\"\n      {{- end }}\n    {{-
    else }}\n      \"no repoURL\"\n    {{- end }}\n}\n{{range $index, $c := .app.status.conditions}}\n
    \ ,\n  {\n    \"name\": \"{{$c.type}}\",\n    \"value\": \"{{$c.message}}\"\n
    \ }\n{{end}}\n]\n"
  potentialAction: "[{\n  \"@type\":\"OpenUri\",\n  \"name\":\"Open Operation\",\n
    \ \"targets\":[{\n    \"os\":\"default\",\n    \"uri\":\"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true\"\n
    \ }]\n},\n{\n  \"@type\":\"OpenUri\",\n  \"name\":\"Open Repository\",\n  \"targets\":[{\n
    \   \"os\":\"default\",\n    \"uri\": \n      {{- if .app.spec.source }}\n        \"{{
    .app.spec.source.repoURL }}\"\n      {{- else if .app.spec.sources }}\n        {{-
    range .app.spec.sources }}\n          \"{{ .repoURL }}\"\n        {{- end }}\n
    \     {{- else }}\n        \"no repoURL\"\n      {{- end }} \n  }]\n}]"
  title: Start syncing application {{.app.metadata.name}}.

```
### app-sync-status-unknown
**definition**:
```yaml
email:
  subject: Application {{.app.metadata.name}} sync status is 'Unknown'
message: |
  {{if eq .serviceType "slack"}}:exclamation:{{end}} Application {{.app.metadata.name}} sync is 'Unknown'.
  Application details: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}.
  {{if ne .serviceType "slack"}}
  {{range $c := .app.status.conditions}}
      * {{$c.message}}
  {{end}}
  {{end}}
slack:
  attachments: "[{\n  \"title\": \"{{ .app.metadata.name}}\",\n  \"title_link\":\"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}\",\n
    \ \"color\": \"#E96D76\",\n  \"fields\": [\n  {\n    \"title\": \"Sync Status\",\n
    \   \"value\": \"{{.app.status.sync.status}}\",\n    \"short\": true\n  },\n  {\n
    \   \"title\": \"Repository\",\n    \"value\": \n      {{- if .app.spec.source
    }}\n        \"{{ .app.spec.source.repoURL }}\"\n      {{- else if .app.spec.sources
    }}\n        {{- range .app.spec.sources }}\n          \"{{ .repoURL }}\"\n        {{-
    end }}\n      {{- else }}\n        \"no repoURL\"\n      {{- end }}\n    \"short\":
    true\n  }\n  {{range $index, $c := .app.status.conditions}}\n  ,\n  {\n    \"title\":
    \"{{$c.type}}\",\n    \"value\": \"{{$c.message}}\",\n    \"short\": true\n  }\n
    \ {{end}}\n  ]\n}]\n"
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: "[{\n  \"name\": \"Sync Status\",\n  \"value\": \"{{.app.status.sync.status}}\"\n},\n{\n
    \ \"name\": \"Repository\",\n  \"value\": \n  {{- if .app.spec.source }}\n    \"{{
    .app.spec.source.repoURL }}\"\n  {{- else if .app.spec.sources }}\n    {{- range
    .app.spec.sources }}\n      \"{{ .repoURL }}\"\n    {{- end }}\n  {{- else }}\n
    \   \"no repoURL\"\n  {{- end }}\n}\n{{range $index, $c := .app.status.conditions}}\n
    \ ,\n  {\n    \"name\": \"{{$c.type}}\",\n    \"value\": \"{{$c.message}}\"\n
    \ }\n{{end}}\n]\n"
  potentialAction: "[{\n  \"@type\":\"OpenUri\",\n  \"name\":\"Open Application\",\n
    \ \"targets\":[{\n    \"os\":\"default\",\n    \"uri\":\"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}\"\n
    \ }]\n},\n{\n  \"@type\":\"OpenUri\",\n  \"name\":\"Open Repository\",\n  \"targets\":[{\n
    \   \"os\":\"default\",\n    \"uri\": \n      {{- if .app.spec.source }}\n        \"{{
    .app.spec.source.repoURL }}\"\n      {{- else if .app.spec.sources }}\n        {{-
    range .app.spec.sources }}\n          \"{{ .repoURL }}\"\n        {{- end }}\n
    \     {{- else }}\n        \"no repoURL\"\n      {{- end }} \n  }]\n}]"
  title: Application {{.app.metadata.name}} sync status is 'Unknown'

```
### app-sync-succeeded
**definition**:
```yaml
email:
  subject: Application {{.app.metadata.name}} has been successfully synced.
message: |
  {{if eq .serviceType "slack"}}:white_check_mark:{{end}} Application {{.app.metadata.name}} has been successfully synced at {{.app.status.operationState.finishedAt}}.
  Sync operation details are available at: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true .
slack:
  attachments: "[{\n  \"title\": \"{{ .app.metadata.name}}\",\n  \"title_link\":\"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}\",\n
    \ \"color\": \"#18be52\",\n  \"fields\": [\n  {\n    \"title\": \"Sync Status\",\n
    \   \"value\": \"{{.app.status.sync.status}}\",\n    \"short\": true\n  },\n  {\n
    \   \"title\": \"Repository\",\n    \"value\": \n      {{- if .app.spec.source
    }}\n        \"{{ .app.spec.source.repoURL }}\"\n      {{- else if .app.spec.sources
    }}\n        {{- range .app.spec.sources }}\n          \"{{ .repoURL }}\"\n        {{-
    end }}\n      {{- else }}\n        \"no repoURL\"\n      {{- end }}\n    \"short\":
    true\n  }\n  {{range $index, $c := .app.status.conditions}}\n  ,\n  {\n    \"title\":
    \"{{$c.type}}\",\n    \"value\": \"{{$c.message}}\",\n    \"short\": true\n  }\n
    \ {{end}}\n  ]\n}]\n"
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: "[{\n  \"name\": \"Sync Status\",\n  \"value\": \"{{.app.status.sync.status}}\"\n},\n{\n
    \ \"name\": \"Synced at\",\n  \"value\": \"{{.app.status.operationState.finishedAt}}\"\n},\n{\n
    \ \"name\": \"Repository\",\n  \"value\": \n    {{- if .app.spec.source }}\n      \"{{
    .app.spec.source.repoURL }}\"\n    {{- else if .app.spec.sources }}\n      {{-
    range .app.spec.sources }}\n        \"{{ .repoURL }}\"\n      {{- end }}\n    {{-
    else }}\n      \"no repoURL\"\n    {{- end }}\n}\n{{range $index, $c := .app.status.conditions}}\n
    \ ,\n  {\n    \"name\": \"{{$c.type}}\",\n    \"value\": \"{{$c.message}}\"\n
    \ }\n{{end}}\n]\n"
  potentialAction: "[{\n  \"@type\":\"OpenUri\",\n  \"name\":\"Operation Details\",\n
    \ \"targets\":[{\n    \"os\":\"default\",\n    \"uri\":\"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true\"\n
    \ }]\n},\n{\n  \"@type\":\"OpenUri\",\n  \"name\":\"Open Repository\",\n  \"targets\":[{\n
    \   \"os\":\"default\",\n    \"uri\": \n      {{- if .app.spec.source }}\n        \"{{
    .app.spec.source.repoURL | call .repo.RepoURLToHTTPS }}\"\n      {{- else if .app.spec.sources
    }}\n        {{- range .app.spec.sources }}\n          \"{{ .repoURL | call .repo.RepoURLToHTTPS
    }}\"\n        {{- end }}\n      {{- else }}\n        \"no repoURL\"\n      {{-
    end }}\n  }]\n}]"
  themeColor: '#000080'
  title: Application {{.app.metadata.name}} has been successfully synced

```
