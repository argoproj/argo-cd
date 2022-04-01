# Extensions

An extension is a way to add new capabilities to the UI.

Use cases include:

* Surfacing high-level application telemetry and insights.
* Recommending Kubernetes best practices.
* Alerting users about vulnerabilities in their application.

* ⚠️ Only install extensions from trusted sources.

## UI Extension

A Javascript module should be:

* Packaged into a single `.js` file.
* Installed on every Argo CD server at `/tmp/extensions/{name}.js`
* Listed in `/tmp/extensions/index.json` {e.g. `{"items": ["name"]}`}

You can add the following types of extension:

* `appToolbarButton` - A button added to the application toolbar.
* `appPanel` - A sliding panel.
* `appStatusPanelItem` - An item added to the application status panel.
* `resourcePanel` - An item added as a resource panel.

Security:

* An extension will be able to make HTTP requests.

See [example](https://github.com/argoproj-labs/argocd-example-extension/v2/ui).
