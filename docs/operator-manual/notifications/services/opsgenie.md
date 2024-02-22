# Opsgenie

To be able to send notifications with argocd-notifications you have to create an [API Integration](https://docs.opsgenie.com/docs/integrations-overview) inside your [Opsgenie Team](https://docs.opsgenie.com/docs/teams).

1. Login to Opsgenie at https://app.opsgenie.com or https://app.eu.opsgenie.com (if you have an account in the european union)
2. Make sure you already have a team, if not follow this guide https://docs.opsgenie.com/docs/teams
3. Click "Teams" in the Menu on the left
4. Select the team that you want to notify
5. In the teams configuration menu select "Integrations"
6. click "Add Integration" in the top right corner
7. Select "API" integration
8. Give your integration a name, copy the "API key" and safe it somewhere for later
9. Make sure the checkboxes for "Create and Update Access" and "enable" are selected, disable the other checkboxes to remove unnecessary permissions
10. Click "Safe Integration" at the bottom
11. Check your browser for the correct server apiURL. If it is "app.opsgenie.com" then use the US/international api url `api.opsgenie.com` in the next step, otherwise use `api.eu.opsgenie.com` (European API). 
12. You are finished with configuring Opsgenie. Now you need to configure argocd-notifications. Use the apiUrl, the team name and the apiKey to configure the Opsgenie integration in the `argocd-notifications-secret` secret.


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
```