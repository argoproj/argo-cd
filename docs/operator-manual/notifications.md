# Notifications

The notifications support is not bundled into the Argo CD itself. Instead of reinventing the wheel and implementing opinionated notifications system Argo CD leverages integrations
with the third-party notification system. Following integrations are recommended:

* To monitor Argo CD performance or health state of managed applications use [Prometheus Metrics](./metrics.md) in combination with [Grafana](https://grafana.com/),
[Alertmanager](https://prometheus.io/docs/alerting/alertmanager/).
* To notify the end-users of  Argo CD about events like application upgrades, user errors in application definition, etc use one of the following projects:
    * [ArgoCD Notifications](https://github.com/argoproj-labs/argocd-notifications) - Argo CD specific notification system that continuously monitors Argo CD applications 
    and aims to integrate with various notification services such as Slack, SMTP, Telegram, Discord, etc.
    * [Argo Kube Notifier](https://github.com/argoproj-labs/argo-kube-notifier) - generic Kubernetes resource controller that allows monitoring any Kubernetes resource and sends a
    notification when the configured rule is met.
    * [Kube Watch](https://github.com/bitnami-labs/kubewatch) - a Kubernetes watcher that could publishes notification to Slack/hipchat/mattermost/flock channels. It watches the
    cluster for resource changes and notifies them through webhooks.
