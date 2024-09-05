# Opsgenie

To be able to send notifications with argocd-notifications you have to create an [API Integration](https://docs.opsgenie.com/docs/integrations-overview) inside your [Opsgenie Team](https://docs.opsgenie.com/docs/teams).

1. Login to Opsgenie at https://app.opsgenie.com or https://app.eu.opsgenie.com (if you have an account in the european union)
2. Make sure you already have a team, if not follow this guide https://docs.opsgenie.com/docs/teams
3. Click "Teams" in the Menu on the left
4. Select the team that you want to notify
5. In the teams configuration menu select "Integrations"
6. Click "Add Integration" in the top right corner
7. Select "API" integration
8. Give your integration a name, copy the "API key" and safe it somewhere for later
9. Click "Edit" in the integration settings
10. Make sure the checkbox for "Create and Update Access" is selected, disable the other checkboxes to remove unnecessary permissions
11. Click "Save" at the bottom
12. Click "Turn on integration" in the top right corner
13. Check your browser for the correct server apiURL. If it is "app.opsgenie.com" then use the US/international api url `api.opsgenie.com` in the next step, otherwise use `api.eu.opsgenie.com` (European API). 
14. You are finished with configuring Opsgenie. Now you need to configure argocd-notifications. Use the apiUrl, the team name and the apiKey to configure the Opsgenie integration in the `argocd-notifications-secret` secret.
15. You can find the example `argocd-notifications-cm` configuration at the below.

| **Option**    | **Required** | **Type** | **Description**                                                                                          | **Example**                      |
| ------------- | ------------ | -------- | -------------------------------------------------------------------------------------------------------- | -------------------------------- |
| `description` | True         | `string` | Description field of the alert that is generally used to provide a detailed information about the alert. | `Hello from Argo CD!`            |
| `priority`    | False        | `string` | Priority level of the alert. Possible values are P1, P2, P3, P4 and P5. Default value is P3.             | `P1`                             |
| `alias`       | False        | `string` | Client-defined identifier of the alert, that is also the key element of Alert De-Duplication.            | `Life is too short for no alias` |
| `note`       | False        | `string` | Additional note that will be added while creating the alert.            | `Error from Argo CD!` |

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.opsgenie: |
    apiUrl: <api-url>
    apiKeys:
      <your-team>: <integration-api-key>
  template.opsgenie: |
    message: |
      [Argo CD] Application {{.app.metadata.name}} has a problem.
    opsgenie:
      description: |
        Application: {{.app.metadata.name}}
        Health Status: {{.app.status.health.status}}
        Operation State Phase: {{.app.status.operationState.phase}}
        Sync Status: {{.app.status.sync.status}}
      priority: P1
      alias: {{.app.metadata.name}}
      note: Error from Argo CD!
  trigger.on-a-problem: |
    - description: Application has a problem.
      send:
      - opsgenie
      when: app.status.health.status == 'Degraded' or app.status.operationState.phase in ['Error', 'Failed'] or app.status.sync.status == 'Unknown'
```

16. Add annotation in application yaml file to enable notifications for specific Argo CD app.
```yaml
  apiVersion: argoproj.io/v1alpha1
  kind: Application
  metadata:
    annotations:
      notifications.argoproj.io/subscribe.on-a-problem.opsgenie: <your-team>
```