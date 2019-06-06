# Ingress Configuration

Argo CD runs both a gRPC server (used by the CLI), as well as a HTTP/HTTPS server (used by the UI).
Both protocols are exposed by the argocd-server service object on the following ports:

* 443 - gRPC/HTTPS
* 80 - HTTP (redirects to HTTPS)

There are several ways how Ingress can be configured.

## [kubernetes/ingress-nginx](https://github.com/kubernetes/ingress-nginx)

### Option 1: SSL-Passthrough

Because Argo CD serves multiple protocols (gRPC/HTTPS) on the same port (443), this provides a
challenge when attempting to define a single nginx ingress object and rule for the argocd-service,
since the `nginx.ingress.kubernetes.io/backend-protocol` [annotation](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/annotations/#backend-protocol)
accepts only a single value for the backend protocol (e.g. HTTP, HTTPS, GRPC, GRPCS).

In order to expose the Argo CD API server with a single ingress rule and hostname, the
`nginx.ingress.kubernetes.io/ssl-passthrough` [annotation](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/annotations/#ssl-passthrough)
must be used to passthrough TLS connections and terminate TLS at the Argo CD API server.

```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: argocd-server-ingress
  annotations:
    kubernetes.io/ingress.class: nginx
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
    nginx.ingress.kubernetes.io/ssl-passthrough: "true"
spec:
  rules:
  - host: argocd.example.com
    http:
      paths:
      - backend:
          serviceName: argocd-server
          servicePort: https
```

The above rule terminates TLS at the Argo CD API server, which detects the protocol being used,
and responds appropriately. Note that the `nginx.ingress.kubernetes.io/ssl-passthrough` annotation
requires that the `--enable-ssl-passthrough` flag be added to the command line arguments to
`nginx-ingress-controller`.

### Option 2: Multiple Ingress Objects And Hosts

Since ingress-nginx Ingress supports only a single protocol per Ingress object, an alternative
way would be to define two Ingress objects. One for HTTP/HTTPS, and the other for gRPC:

HTTP/HTTPS Ingress:
```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: argocd-server-http-ingress
  annotations:
    kubernetes.io/ingress.class: "nginx"
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
    nginx.ingress.kubernetes.io/backend-protocol: "HTTP"
spec:
  rules:
  - http:
      paths:
      - backend:
          serviceName: argocd-server
          servicePort: http
    host: argocd.example.com
  tls:
  - hosts:
    - argocd.example.com
    secretName: argocd-secret
```

gRPC Ingress:
```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: argocd-server-grpc-ingress
  annotations:
    kubernetes.io/ingress.class: "nginx"
    nginx.ingress.kubernetes.io/backend-protocol: "GRPC"
spec:
  rules:
  - http:
      paths:
      - backend:
          serviceName: argocd-server
          servicePort: https
    host: grpc.argocd.example.com
  tls:
  - hosts:
    - grpc.argocd.example.com
    secretName: argocd-secret
```

The API server should then be run with TLS disabled. Edit the `argocd-server` deployment to add the
`--insecure` flag to the argocd-server command:

```yaml
spec:
  template:
    spec:
      name: argocd-server
      containers:
      - command:
        - /argocd-server
        - --staticassets
        - /shared/app
        - --repo-server
        - argocd-repo-server:8081
        - --insecure
```

The obvious disadvantage to this approach is that this technique require two separate hostnames for
the API server -- one for gRPC and the other for HTTP/HTTPS. However it allow TLS termination to
happen at the ingress controller.


## AWS Application Load Balancers (ALBs) And Classic ELB (HTTP Mode)

Neither ALBs and Classic ELB in HTTP mode, do not have full support for HTTP2/gRPC which is the
protocol used by the `argocd` CLI. Thus, when using an AWS load balancer, either Classic ELB in
passthrough mode is needed, or NLBs.

```shell
$ argocd login <host>:<port> --grpc-web
```


## UI Base Path

If Argo CD UI is available under non-root path (e.g. `/argo-cd` instead of `/`) then UI path should be configured in API server.
To configure UI path add `--basehref` flag into `argocd-server` deployment command:

```yaml
spec:
  template:
    spec:
      name: argocd-server
      containers:
      - command:
        - /argocd-server
        - --staticassets
        - /shared/app
        - --repo-server
        - argocd-repo-server:8081
        - --basehref
        - /argo-cd
```

NOTE: flag `--basehref` only changes UI base URL. API server keep using `/` path so you need to add URL rewrite rule to proxy config.
Example nginx.conf with URL rewrite:

```
worker_processes 1;

events { worker_connections 1024; }

http {

    sendfile on;

    server {
        listen 443;

        location /argo-cd {
            rewrite /argo-cd/(.*) /$1  break;
            proxy_pass         https://localhost:8080;
            proxy_redirect     off;
            proxy_set_header   Host $host;
            proxy_set_header   X-Real-IP $remote_addr;
            proxy_set_header   X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header   X-Forwarded-Host $server_name;
        }
    }
}
```
