# Status Badge

> v1.2

Argo CD can display a badge with health and sync status for any application.

![healthy and synced](../assets/status-badge-healthy-synced.png)

To show this badge, use the following URL format `${argoCdBaseUrl}/api/badge?name=${appName}`, e.g. http://localhost:8080/api/badge?name=guestbook