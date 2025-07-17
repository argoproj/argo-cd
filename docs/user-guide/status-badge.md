# Status Badge

Argo CD can display a badge with health and sync status for any application. The feature is disabled by default because badge image is available to any user without authentication.
The feature can be enabled using `statusbadge.enabled` key of `argocd-cm` ConfigMap (see [argocd-cm.yaml](../operator-manual/argocd-cm.yaml)).

![healthy and synced](../assets/status-badge-healthy-synced.png)

To show this badge, use the following URL format `${argoCdBaseUrl}/api/badge?name=${appName}`, e.g. http://localhost:8080/api/badge?name=guestbook.
The URLs for status image are available on application details page:

1. Navigate to application details page and click on 'Details' button.
2. Scroll down to 'Status Badge' section.
3. Select required template such as URL, Markdown etc.
for the status image URL in markdown, html, etc are available .
4. Copy the text and paste it into your README or website.

## Additional query parameters options
### showAppName
Display the application name in the status badge.   

Available values: `true/false`

Default value: `false`

Example: `&showAppName=true`

### revision
Display revision targeted by the application.

It will also extend the badge width to 192px.

Available values: `true/false`

Default value: `false`

Example: `&revision=true`
### keepFullRevision
By default, displayed  revision is truncated to 7 characters.

This parameter allows to display it fully if it exceeds that length.

It will also extend the badge width to 400px.

Available values: `true/false`

Default value: `false`

Example: `&keepFullRevision=true`
### width
Change width of the badge.

Completely replace current calculated width.

Available values: `integer`

Default value: `nil`

Example: `&width=500`