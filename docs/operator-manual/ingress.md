# Ingress Configuration

Argo CD runs both a gRPC server (used by the CLI), as well as a HTTP/HTTPS server (used by the UI).
Both protocols are exposed by the argocd-server service object on the following ports:

* 443 - gRPC/HTTPS
* 80 - HTTP (redirects to HTTPS)

There are several ways how Ingress can be configured.

## [Ambassador](https://www.getambassador.io/)

The Ambassador Edge Stack can be used as a Kubernetes ingress controller with [automatic TLS termination](https://www.getambassador.io/docs/latest/topics/running/tls/#host) and routing capabilities for both the CLI and the UI.

The API server should be run with TLS disabled. Edit the `argocd-server` deployment to add the `--insecure` flag to the argocd-server command. Given the `argocd` CLI includes the port number in the request `host` header, 2 Mappings are required.

### Option 1: Mapping CRD for Host-based Routing
```yaml
apiVersion: getambassador.io/v2
kind: Mapping
metadata:
  name: argocd-server-ui
  namespace: argocd
spec:
  host: argocd.example.com
  prefix: /
  service: argocd-server:443
---
apiVersion: getambassador.io/v2
kind: Mapping
metadata:
  name: argocd-server-cli
  namespace: argocd
spec:
  host: argocd.example.com:443
  prefix: /
  service: argocd-server:443
```

Login with the `argocd` CLI using the extra `--grpc-web-root-path` flag for gRPC-web.

```shell
argocd login <host>:<port> --grpc-web-root-path /
```

### Option 2: Mapping CRD for Path-based Routing

The API server must be configured to be available under a non-root path (e.g. `/argo-cd`). Edit the `argocd-server` deployment to add the `--rootpath=/argo-cd` flag to the argocd-server command.

```yaml
apiVersion: getambassador.io/v2
kind: Mapping
metadata:
  name: argocd-server
  namespace: argocd
spec:
  prefix: /argo-cd
  rewrite: /argo-cd
  service: argocd-server:443
```

Login with the `argocd` CLI using the extra `--grpc-web-root-path` flag for non-root paths.

```shell
argocd login <host>:<port> --grpc-web-root-path /argo-cd
```


## [kubernetes/ingress-nginx](https://github.com/kubernetes/ingress-nginx)

### Option 1: SSL-Passthrough

Argo CD serves multiple protocols (gRPC/HTTPS) on the same port (443), this provides a
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
  namespace: argocd
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

#### SSL-Passthrough with cert-manager and Let's Encrypt

```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: argocd-server-ingress
  namespace: argocd
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    kubernetes.io/ingress.class: nginx
    kubernetes.io/tls-acme: "true"
    nginx.ingress.kubernetes.io/ssl-passthrough: "true"
    # If you encounter a redirect loop or are getting a 307 response code 
    # then you need to force the nginx ingress to connect to the backend using HTTPS.
    #
    # nginx.ingress.kubernetes.io/backend-protocol: "HTTPS"
spec:
  rules:
  - host: argocd.example.com
    http:
      paths:
      - backend:
          serviceName: argocd-server
          servicePort: https
        path: /
  tls:
  - hosts:
    - argocd.example.com
    secretName: argocd-secret # do not change, this is provided by Argo CD
```

### Option 2: Multiple Ingress Objects And Hosts

Since ingress-nginx Ingress supports only a single protocol per Ingress object, an alternative
way would be to define two Ingress objects. One for HTTP/HTTPS, and the other for gRPC:

HTTP/HTTPS Ingress:
```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: argocd-server-http-ingress
  namespace: argocd
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
    secretName: argocd-secret # do not change, this is provided by Argo CD
```

gRPC Ingress:
```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: argocd-server-grpc-ingress
  namespace: argocd
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
    secretName: argocd-secret # do not change, this is provided by Argo CD
```

The API server should then be run with TLS disabled. Edit the `argocd-server` deployment to add the
`--insecure` flag to the argocd-server command:

```yaml
spec:
  template:
    spec:
      containers:
      - name: argocd-server
        command:
        - /argocd-server
        - --staticassets
        - /shared/app
        - --repo-server
        - argocd-repo-server:8081
        - --insecure
```

The obvious disadvantage to this approach is that this technique requires two separate hostnames for
the API server -- one for gRPC and the other for HTTP/HTTPS. However it allows TLS termination to
happen at the ingress controller.


## [Traefik (v2.2)](https://docs.traefik.io/)

Traefik can be used as an edge router and provide [TLS](https://docs.traefik.io/user-guides/grpc/) termination within the same deployment.

It currently has an advantage over NGINX in that it can terminate both TCP and HTTP connections _on the same port_ meaning you do not require multiple hosts or paths.

The API server should be run with TLS disabled. Edit the `argocd-server` deployment to add the `--insecure` flag to the argocd-server command.

### IngressRoute CRD
```yaml
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: argocd-server
  namespace: argocd
spec:
  entryPoints:
    - websecure
  routes:
    - kind: Rule
      match: Host(`argocd.example.com`)
      priority: 10
      services:
        - name: argocd-server
          port: 80
    - kind: Rule
      match: Host(`argocd.example.com`) && Headers(`Content-Type`, `application/grpc`)
      priority: 11
      services:
        - name: argocd-server
          port: 80
          scheme: h2c
  tls:
    certResolver: default
    options: {}
```

## AWS Application Load Balancers (ALBs) And Classic ELB (HTTP Mode)

ALBs and Classic ELBs don't fully support HTTP2/gRPC, which is used by the `argocd` CLI.
Thus, when using an AWS load balancer, either Classic ELB in
passthrough mode is needed, or NLBs.

```shell
$ argocd login <host>:<port> --grpc-web
```

## Authenticating through multiple layers of authenticating reverse proxies

ArgoCD endpoints may be protected by one or more reverse proxies layers, in that case, you can provide additional headers through the `argocd` CLI `--header` parameter to authenticate through those layers.

```shell
$ argocd login <host>:<port> --header 'x-token1:foo' --header 'x-token2:bar' # can be repeated multiple times
$ argocd login <host>:<port> --header 'x-token1:foo,x-token2:bar' # headers can also be comma separated
```
## ArgoCD Server and UI Root Path (v1.5.3)

ArgoCD server and UI can be configured to be available under a non-root path (e.g. `/argo-cd`).
To do this, add the `--rootpath` flag into the `argocd-server` deployment command:

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
        - --rootpath
        - /argo-cd
```
NOTE: The flag `--rootpath` changes both API Server and UI base URL. 
Example nginx.conf:

```
worker_processes 1;

events { worker_connections 1024; }

http {

    sendfile on;

    server {
        listen 443;

        location /argo-cd/ {
            proxy_pass         https://localhost:8080/argo-cd/;
            proxy_redirect     off;
            proxy_set_header   Host $host;
            proxy_set_header   X-Real-IP $remote_addr;
            proxy_set_header   X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header   X-Forwarded-Host $server_name;
            # buffering should be disabled for api/v1/stream/applications to support chunked response
            proxy_buffering off;
        }
    }
}
```
Flag ```--grpc-web-root-path ``` is used to provide a non-root path (e.g. /argo-cd)

```shell
$ argocd login <host>:<port> --grpc-web-root-path /argo-cd
```

## UI Base Path

If the Argo CD UI is available under a non-root path (e.g. `/argo-cd` instead of `/`) then the UI path should be configured in the API server.
To configure the UI path add the `--basehref` flag into the `argocd-server` deployment command:

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

NOTE: The flag `--basehref` only changes the UI base URL. The API server will keep using the `/` path so you need to add a URL rewrite rule to the proxy config.
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
            # buffering should be disabled for api/v1/stream/applications to support chunked response
            proxy_buffering off;
        }
    }
}
```
