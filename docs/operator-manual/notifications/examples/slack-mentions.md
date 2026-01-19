# Slack Mentions with Commit Authors

This guide shows how to configure ArgoCD notifications to mention Slack users based on commit authors, especially useful for auto-sync scenarios.

## Problem

When using auto-sync, the `{{.app.status.operationState.operation.initiatedBy.username}}` field is empty because there's no manual user who triggered the sync. Additionally, if multiple users commit changes between syncs, you want to notify all of them, not just the author of the latest commit.

## Solution

Use the new `GetCommitAuthorsFromPreviousSync()` function to get all unique commit authors between the previous sync and current sync.

## Basic Example

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  template.app-sync-succeeded: |
    webhook:
      slack:
        method: POST
        body: |
          {
            "channel": "#test-slack",
            "attachments": [{
              "title": "{{.app.metadata.name}}",
              "title_link": "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
              "color": "#18be52",
              "fields": [
                {
                  "title": "Authors",
                  "value": "{{ $authors := call .repo.GetCommitAuthorsFromPreviousSync }}{{ if $authors }}{{ join $authors ", " }}{{ else }}{{ if .app.status.operationState.operation.initiatedBy.username }}{{ .app.status.operationState.operation.initiatedBy.username }}{{ else }}Auto-sync{{ end }}{{ end }}",
                  "short": true
                },
                {
                  "title": "Repository",
                  "value": "{{.app.spec.source.repoURL}}",
                  "short": true
                }
              ]
            }]
          }
```

## Advanced Example with Email Extraction

If you want to extract just email addresses for mapping to Slack user IDs:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  template.app-sync-succeeded: |
    webhook:
      slack:
        method: POST
        body: |
          {
            "channel": "#test-slack",
            "text": "Application {{.app.metadata.name}} synced successfully",
            "attachments": [{
              "title": "{{.app.metadata.name}}",
              "title_link": "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
              "color": "#18be52",
              "fields": [
                {
                  "title": "Commit Authors",
                  "value": "{{ $authors := call .repo.GetCommitAuthorsFromPreviousSync }}{{ if $authors }}{{ range $idx, $author := $authors }}{{ if $idx }}, {{ end }}{{ call $.repo.ExtractEmailFromAuthor $author }}{{ end }}{{ else }}No new commits{{ end }}",
                  "short": false
                }
              ]
            }]
          }
```

## Mapping Emails to Slack User IDs

To create actual Slack mentions (e.g., `<@U123456>`), you'll need to map email addresses to Slack user IDs. You can do this in your template using a custom mapping:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  template.app-sync-succeeded: |
    webhook:
      slack:
        method: POST
        body: |
          {
            "channel": "#test-slack",
            "text": "{{ $authors := call .repo.GetCommitAuthorsFromPreviousSync }}{{ if $authors }}{{ range $author := $authors }}{{ $email := call $.repo.ExtractEmailFromAuthor $author }}{{ if eq $email "user1@example.com" }}<@U123456>{{ else if eq $email "user2@example.com" }}<@U789012>{{ else }}{{ $email }}{{ end }} {{ end }}{{ else }}Application {{.app.metadata.name}} synced{{ end }}",
            "attachments": [{
              "title": "{{.app.metadata.name}}",
              "title_link": "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
              "color": "#18be52"
            }]
          }
```

## Using Between Specific Revisions

If you need to get authors between specific revisions (not just previous sync), use `GetCommitAuthorsBetween`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  template.app-sync-succeeded: |
    webhook:
      slack:
        method: POST
        body: |
          {
            "channel": "#test-slack",
            "text": "Authors: {{ join (call .repo.GetCommitAuthorsBetween "abc123" "def456") ", " }}"
          }
```

## Available Functions

- `GetCommitAuthorsFromPreviousSync()` - Returns all unique authors between previous and current sync
- `GetCommitAuthorsBetween(fromRevision, toRevision)` - Returns all unique authors between two specific revisions
- `ExtractEmailFromAuthor(author)` - Extracts email from "Name <email>" format
- `FormatSlackMentions(authors)` - Helper to format authors as comma-separated emails

## Notes

1. The functions return authors in "Name <email>" format
2. Authors are deduplicated automatically
3. Merge commits are excluded from the results
4. If there's no previous sync or revisions are the same, an empty list is returned
5. For multisource applications, the first source's revision is used
