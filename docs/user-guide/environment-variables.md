# Environment Variables

The following environment variables can be used with `argocd` CLI:

| Environment Variable                 | Description                                                                                                                                                                                               |
| ------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `ARGOCD_SERVER`                      | the address of the Argo CD server without `https://` prefix <br> (instead of specifying `--server` for every command) <br> eg. `ARGOCD_SERVER=argocd.example.com` if served through an ingress with DNS   |
| `ARGOCD_AUTH_TOKEN`                  | the Argo CD `apiKey` for your Argo CD user to be able to authenticate                                                                                                                                     |
| `ARGOCD_OPTS`                        | command-line options to pass to `argocd` CLI <br> eg. `ARGOCD_OPTS="--grpc-web"`                                                                                                                          |
| `ARGOCD_CONFIG_DIR`                  | sets the directory hosting `argocd` CLI config, e.g., `~/.config/argocd/config`. (if ENV var not provided, defaults to `$XDG_CONFIG_HOME/argocd`, or `~/.config/argocd`, or if exists legacy `~/.argocd`) |
| `ARGOCD_SERVER_NAME`                 | the Argo CD API Server name (default "argocd-server")                                                                                                                                                     |
| `ARGOCD_REPO_SERVER_NAME`            | the Argo CD Repository Server name (default "argocd-repo-server")                                                                                                                                         |
| `ARGOCD_APPLICATION_CONTROLLER_NAME` | the Argo CD Application Controller name (default "argocd-application-controller")                                                                                                                         |
| `ARGOCD_REDIS_NAME`                  | the Argo CD Redis name (default "argocd-redis")                                                                                                                                                           |
| `ARGOCD_REDIS_HAPROXY_NAME`          | the Argo CD Redis HA Proxy name (default "argocd-redis-ha-haproxy")                                                                                                                                       |
| `ARGOCD_REDIS_KEY_PREFIX`            | the Argo CD Redis keys prefix (default "")
|
| `ARGOCD_GRPC_KEEP_ALIVE_MIN`         | defines the GRPCKeepAliveEnforcementMinimum, used in the grpc.KeepaliveEnforcementPolicy. Expects a "Duration" format (default `10s`).                                                                    |
