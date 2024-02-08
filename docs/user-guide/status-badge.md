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

To be able to see the application name in the application status badge use the `?showAppName=true` query parameter.   
For instance, `${argoCdBaseUrl}/api/badge?name=${appName}&showAppName=true`.   
Naturally, to remove the application name from the badge, remove the query parameter from the URL or set it to `false`.