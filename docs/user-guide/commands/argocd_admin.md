# `argocd admin` Command Reference

## argocd admin

Contains a set of commands useful for Argo CD administrators and requires direct Kubernetes access

```
argocd admin [flags]
```

### Examples

```
# List all clusters
$ argocd admin cluster list

# Add a new cluster
$ argocd admin cluster add my-cluster --name my-cluster --in-cluster-context

# Remove a cluster
argocd admin cluster remove my-cluster

# List all projects
$ argocd admin project list

# Create a new project
$argocd admin project create my-project --src-namespace my-source-namespace --dest-namespace my-dest-namespace

# Update a project
$ argocd admin project update my-project --src-namespace my-updated-source-namespace --dest-namespace my-updated-dest-namespace

# Delete a project
$ argocd admin project delete my-project

# List all settings
$ argocd admin settings list

# Get the current settings
$ argocd admin settings get

# Update settings
$ argocd admin settings update --repository.resync --value 15

# List all applications
$ argocd admin app list

# Get application details
$ argocd admin app get my-app

# Sync an application
$ argocd admin app sync my-app

# Pause an application
$ argocd admin app pause my-app

# Resume an application
$ argocd admin app resume my-app

# List all repositories
$ argocd admin repo list

# Add a repository
$ argocd admin repo add https://github.com/argoproj/my-repo.git

# Remove a repository
$ argocd admin repo remove https://github.com/argoproj/my-repo.git

# Import an application from a YAML file
$ argocd admin app import -f my-app.yaml

# Export an application to a YAML file
$ argocd admin app export my-app -o my-exported-app.yaml

# Access the Argo CD web UI
$ argocd admin dashboard

# List notifications
$ argocd admin notification list

# Get notification details
$ argocd admin notification get my-notification

# Create a new notification
$ argocd admin notification create my-notification -f notification-config.yaml

# Update a notification
$ argocd admin notification update my-notification -f updated-notification-config.yaml

# Delete a notification
$ argocd admin notification delete my-notification

# Reset the initial admin password
$ argocd admin initial-password reset

```

### Options

```
  -h, --help               help for admin
      --logformat string   Set the logging format. One of: text|json (default "text")
      --loglevel string    Set the logging level. One of: debug|info|warn|error (default "info")
```

### Options inherited from parent commands

```
      --auth-token string               Authentication token
      --client-crt string               Client certificate file
      --client-crt-key string           Client certificate key file
      --config string                   Path to Argo CD config (default "/home/user/.config/argocd/config")
      --controller-name string          Name of the Argo CD Application controller; set this or the ARGOCD_APPLICATION_CONTROLLER_NAME environment variable when the controller's name label differs from the default, for example when installing via the Helm chart (default "argocd-application-controller")
      --core                            If set to true then CLI talks directly to Kubernetes instead of talking to Argo CD API server
      --grpc-web                        Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2.
      --grpc-web-root-path string       Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2. Set web root.
  -H, --header strings                  Sets additional header to all requests made by Argo CD CLI. (Can be repeated multiple times to add multiple headers, also supports comma separated headers)
      --http-retry-max int              Maximum number of retries to establish http connection to Argo CD server
      --insecure                        Skip server certificate and domain verification
      --kube-context string             Directs the command to the given kube-context
      --plaintext                       Disable TLS
      --port-forward                    Connect to a random argocd-server port using port forwarding
      --port-forward-namespace string   Namespace name which should be used for port forwarding
      --redis-haproxy-name string       Name of the Redis HA Proxy; set this or the ARGOCD_REDIS_HAPROXY_NAME environment variable when the HA Proxy's name label differs from the default, for example when installing via the Helm chart (default "argocd-redis-ha-haproxy")
      --redis-name string               Name of the Redis deployment; set this or the ARGOCD_REDIS_NAME environment variable when the Redis's name label differs from the default, for example when installing via the Helm chart (default "argocd-redis")
      --repo-server-name string         Name of the Argo CD Repo server; set this or the ARGOCD_REPO_SERVER_NAME environment variable when the server's name label differs from the default, for example when installing via the Helm chart (default "argocd-repo-server")
      --server string                   Argo CD server address
      --server-crt string               Server certificate file
      --server-name string              Name of the Argo CD API server; set this or the ARGOCD_SERVER_NAME environment variable when the server's name label differs from the default, for example when installing via the Helm chart (default "argocd-server")
```

### SEE ALSO

* [argocd](argocd.md)	 - argocd controls a Argo CD server
* [argocd admin app](argocd_admin_app.md)	 - Manage applications configuration
* [argocd admin cluster](argocd_admin_cluster.md)	 - Manage clusters configuration
* [argocd admin dashboard](argocd_admin_dashboard.md)	 - Starts Argo CD Web UI locally
* [argocd admin export](argocd_admin_export.md)	 - Export all Argo CD data to stdout (default) or a file
* [argocd admin import](argocd_admin_import.md)	 - Import Argo CD data from stdin (specify `-') or a file
* [argocd admin initial-password](argocd_admin_initial-password.md)	 - Prints initial password to log in to Argo CD for the first time
* [argocd admin notifications](argocd_admin_notifications.md)	 - Set of CLI commands that helps manage notifications settings
* [argocd admin proj](argocd_admin_proj.md)	 - Manage projects configuration
* [argocd admin repo](argocd_admin_repo.md)	 - Manage repositories configuration
* [argocd admin settings](argocd_admin_settings.md)	 - Provides set of commands for settings validation and troubleshooting

