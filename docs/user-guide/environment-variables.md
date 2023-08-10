# Environment Variables

The following environment variables can be used with `argocd` CLI:

| Environment Variable | Description |
| --- | --- |
| `ARGOCD_SERVER` | the address of the Argo CD server without `https://` prefix <br> (instead of specifying `--server` for every command) <br> eg. `ARGOCD_SERVER=argocd.mycompany.com` if served through an ingress with DNS |
| `ARGOCD_AUTH_TOKEN` | the Argo CD `apiKey` for your Argo CD user to be able to authenticate |
| `ARGOCD_OPTS` | command-line options to pass to `argocd` CLI <br> eg. `ARGOCD_OPTS="--grpc-web"` |
