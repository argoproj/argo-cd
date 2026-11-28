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

## Browser for SSO login

`argocd login --sso` and `argocd relogin` open the SSO URL with the command in `BROWSER` when it is set, and with the
system default browser otherwise:

- `BROWSER` holds **one** command, split into arguments with shell quoting rules. A list of commands separated by
  `:` (as `xdg-open` accepts) is not supported.
- An argument that is exactly `%s` is replaced by the URL; otherwise the URL is appended. `%s` inside a larger
  argument, such as `--app=%s`, is not replaced.
- On Windows, put a path that holds backslashes in single quotes, or write it with forward slashes. `cmd.exe` and
  `.bat` / `.cmd` files are refused, because `cmd.exe` would run whatever follows a `&` in the URL.
- If the command cannot be used, the CLI logs a warning and opens the system default browser. If no browser opens,
  the login goes on: open the printed URL by hand, or pass `--sso-launch-browser=false`.

```shell
export BROWSER='open -na "Google Chrome" --args --profile-directory="Profile 1" %s'
```

## Proxy environment variables

For gRPC-web connections, the `argocd` CLI honors the standard `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY` environment
variables (and their lowercase equivalents), following Go's
[`http.ProxyFromEnvironment`](https://pkg.go.dev/net/http#ProxyFromEnvironment) rules. Connections to the API server
use HTTPS unless `--plaintext` is given, so `HTTPS_PROXY` is normally the variable that applies. Requests to
`localhost` and loopback addresses are never proxied, and hosts matched by `NO_PROXY` bypass the proxy.

Plain gRPC connections, i.e. those made without `--grpc-web`, do not use `HTTP_PROXY`/`HTTPS_PROXY`. They only support
a SOCKS5 proxy configured through `ALL_PROXY`. If your environment provides an HTTP proxy only, pass `--grpc-web` or
set `ARGOCD_OPTS="--grpc-web"`.
