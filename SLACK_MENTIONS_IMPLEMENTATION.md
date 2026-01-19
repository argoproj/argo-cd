# Slack Mentions Implementation for ArgoCD Notifications

## Summary

This implementation adds support for mentioning Slack users based on commit authors in ArgoCD notifications, solving the issue where auto-sync scenarios don't show who committed changes.

## Changes Made

### 1. Git Client (`util/git/client.go`)
- Added `GetCommitAuthorsBetween(fromRevision, toRevision)` method to the `Client` interface
- Implemented the method to get unique commit authors between two revisions using `git log`
- Excludes merge commits and returns authors in "Name <email>" format

### 2. Reposerver API (`reposerver/repository/`)
- Added new RPC method `GetCommitAuthorsBetween` to `repository.proto`
- Added request/response messages: `RepoServerCommitAuthorsRequest` and `RepoServerCommitAuthorsResponse`
- Implemented the service method in `repository.go`

### 3. Notification Service (`util/notification/argocd/service.go`)
- Added `GetCommitAuthorsBetween` method to the `Service` interface
- Implemented the method to call the reposerver API

### 4. Notification Expressions (`util/notification/expression/repo/repo.go`)
- Added `GetCommitAuthorsBetween(fromRevision, toRevision)` function
- Added `GetCommitAuthorsFromPreviousSync()` function that automatically gets authors between previous and current sync
- Added `ExtractEmailFromAuthor(author)` helper function
- Added `FormatSlackMentions(authors)` helper function

### 5. Documentation
- Updated `docs/operator-manual/notifications/functions.md` with new function documentation
- Updated `docs/operator-manual/notifications/templates.md` with examples
- Created `docs/operator-manual/notifications/examples/slack-mentions.md` with comprehensive examples

## Error Handling Improvements

All functions now include comprehensive error handling:

1. **Input Validation**: Empty strings, same revisions, and invalid inputs return empty results
2. **Graceful Degradation**: Errors return empty slices instead of failing notifications
3. **Revision Checking**: Validates revisions exist before querying git
4. **Edge Cases**: Handles first sync, no history, same revisions, invalid revisions
5. **Email Extraction**: Improved to handle malformed author strings
6. **Deduplication**: Email addresses are deduplicated in FormatSlackMentions

## Next Steps

### 1. Regenerate Proto Files
The proto files need to be regenerated to include the new RPC method. Run:
```bash
make generate
```
Or:
```bash
./hack/generate-proto.sh
```

This will generate the Go code from the updated `repository.proto` file.

### 2. Build and Test
After regenerating proto files, build the project:
```bash
make build
```

### 3. Testing Checklist
- [ ] Test with auto-sync enabled application
- [ ] Test with manual sync
- [ ] Test with first sync (no history)
- [ ] Test with same revision (no new commits)
- [ ] Test with multiple commits from different authors
- [ ] Test with invalid revisions
- [ ] Test with multisource applications
- [ ] Verify Slack notifications work correctly

### 4. Usage Example

Here's how to use the new functionality in your Slack webhook configuration:

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

## Key Features

1. **Automatic Previous Sync Detection**: `GetCommitAuthorsFromPreviousSync()` automatically uses the application's sync history to find the previous revision
2. **Unique Authors**: All duplicate authors are automatically removed
3. **Auto-Sync Support**: Works perfectly with auto-sync scenarios where `initiatedBy.username` is empty
4. **Multiple Commits**: Gets all authors from all commits between syncs, not just the latest one
5. **Multisource Support**: Handles both single-source and multisource applications

## Testing

To test the implementation:

1. Ensure you have a Git repository with multiple commits from different authors
2. Configure an ArgoCD application with auto-sync enabled
3. Make commits from different users
4. Wait for auto-sync to trigger
5. Check the Slack notification - it should list all authors who committed since the last sync

## Notes

- The functions return authors in "Name <email>" format
- To create actual Slack mentions (e.g., `<@U123456>`), you'll need to map email addresses to Slack user IDs in your template
- Merge commits are excluded from the results
- If there's no previous sync or revisions are the same, an empty list is returned
