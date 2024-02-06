# Ingress Configuration

Argo CD API server runs both a gRPC server (used by the CLI), as well as a HTTP/HTTPS server (used by the UI).
Both protocols are exposed by the argocd-server service object on the following ports:

* 443 - gRPC/HTTPS
* 80 - HTTP (redirects to HTTPS)

There are several ways how Ingress can be configured.

## [Ambassador](https://www.getambassador.io/)

The Ambassador Edge Stack can be used as a Kubernetes ingress controller with [automatic TLS termination](https://www.getambassador.io/docs/latest/topics/running/tls/#host) and routing capabilities for both the CLI and the UI.

The API server should be run with TLS disabled. Edit the `argocd-server` deployment to add the `--insecure` flag to the argocd-server command, or simply set `server.insecure: "true"` in the `argocd-cmd-params-cm` ConfigMap [as described here](server-commands/additional-configuration-method.md). Given the `argocd` CLI includes the port number in the request `host` header, 2 Mappings are required.

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
  # NOTE: the port must be ignored if you have strip_matching_host_port enabled on envoy
  host: argocd.example.com:443
  prefix: /
  service: argocd-server:80
  regex_headers:
    Content-Type: "^application/grpc.*$"
  grpc: true
```

Login with the `argocd` CLI:

```shell
argocd login <host>
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

## [Contour](https://projectcontour.io/)
The Contour ingress controller can terminate TLS ingress traffic at the edge.

The Argo CD API server should be run with TLS disabled. Edit the `argocd-server` Deployment to add the `--insecure` flag to the argocd-server container command, or simply set `server.insecure: "true"` in the `argocd-cmd-params-cm` ConfigMap [as described here](server-commands/additional-configuration-method.md).

It is also possible to provide an internal-only ingress path and an external-only ingress path by deploying two instances of Contour: one behind a private-subnet LoadBalancer service and one behind a public-subnet LoadBalancer service. The private Contour deployment will pick up Ingresses annotated with `kubernetes.io/ingress.class: contour-internal` and the public Contour deployment will pick up Ingresses annotated with `kubernetes.io/ingress.class: contour-external`.

This provides the opportunity to deploy the Argo CD UI privately but still allow for SSO callbacks to succeed.

### Private Argo CD UI with  Multiple Ingress Objects and BYO Certificate
Since Contour Ingress supports only a single protocol per Ingress object, define three Ingress objects. One for private HTTP/HTTPS, one for private gRPC, and one for public HTTPS SSO callbacks.

Internal HTTP/HTTPS Ingress:
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: argocd-server-http
  annotations:
    kubernetes.io/ingress.class: contour-internal
    ingress.kubernetes.io/force-ssl-redirect: "true"
spec:
  rules:
  - host: internal.path.to.argocd.io
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: argocd-server
            port:
              name: http
  tls:
  - hosts:
    - internal.path.to.argocd.io
    secretName: your-certificate-name
```

Internal gRPC Ingress:
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: argocd-server-grpc
  annotations:
    kubernetes.io/ingress.class: contour-internal
spec:
  rules:
  - host: grpc-internal.path.to.argocd.io
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: argocd-server
            port:
              name: https
  tls:
  - hosts:
    - grpc-internal.path.to.argocd.io
    secretName: your-certificate-name
```

External HTTPS SSO Callback Ingress:
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: argocd-server-external-callback-http
  annotations:
    kubernetes.io/ingress.class: contour-external
    ingress.kubernetes.io/force-ssl-redirect: "true"
spec:
  rules:
  - host: external.path.to.argocd.io
    http:
      paths:
      - path: /api/dex/callback
        pathType: Prefix
        backend:
          service:
            name: argocd-server
            port:
              name: http
  tls:
  - hosts:
    - external.path.to.argocd.io
    secretName: your-certificate-name
```

The argocd-server Service needs to be annotated with `projectcontour.io/upstream-protocol.h2c: "https,443"` to wire up the gRPC protocol proxying.

The API server should then be run with TLS disabled. Edit the `argocd-server` deployment to add the
`--insecure` flag to the argocd-server command, or simply set `server.insecure: "true"` in the `argocd-cmd-params-cm` ConfigMap [as described here](server-commands/additional-configuration-method.md).

Contour httpproxy CRD:

Using a contour httpproxy CRD allows you to use the same hostname for the GRPC and REST api.

```yaml
apiVersion: projectcontour.io/v1
kind: HTTPProxy
metadata:
  name: argocd-server
  namespace: argocd
spec:
  ingressClassName: contour
  virtualhost:
    fqdn: path.to.argocd.io
    tls:
      secretName: wildcard-tls
  routes:
    - conditions:
        - prefix: /
        - header:
            name: Content-Type
            contains: application/grpc
      services:
        - name: argocd-server
          port: 80
          protocol: h2c # allows for unencrypted http2 connections
      timeoutPolicy:
        response: 1h
        idle: 600s
        idleConnection: 600s
    - conditions:
        - prefix: /
      services:
        - name: argocd-server
          port: 80
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
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: argocd-server-ingress
  namespace: argocd
  annotations:
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
    nginx.ingress.kubernetes.io/ssl-passthrough: "true"
spec:
  ingressClassName: nginx
  rules:
  - host: argocd.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: argocd-server
            port:
              name: https
```

The above rule terminates TLS at the Argo CD API server, which detects the protocol being used,
and responds appropriately. Note that the `nginx.ingress.kubernetes.io/ssl-passthrough` annotation
requires that the `--enable-ssl-passthrough` flag be added to the command line arguments to
`nginx-ingress-controller`.

#### SSL-Passthrough with cert-manager and Let's Encrypt

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: argocd-server-ingress
  namespace: argocd
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-passthrough: "true"
    # If you encounter a redirect loop or are getting a 307 response code
    # then you need to force the nginx ingress to connect to the backend using HTTPS.
    #
    nginx.ingress.kubernetes.io/backend-protocol: "HTTPS"
spec:
  ingressClassName: nginx
  rules:
  - host: argocd.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: argocd-server
            port:
              name: https
  tls:
  - hosts:
    - argocd.example.com
    secretName: argocd-server-tls # as expected by argocd-server
```

### Option 2: SSL Termination at Ingress Controller

An alternative approach is to perform the SSL termination at the Ingress. Since an `ingress-nginx` Ingress supports only a single protocol per Ingress object, two Ingress objects need to be defined using the `nginx.ingress.kubernetes.io/backend-protocol` annotation, one for HTTP/HTTPS and the other for gRPC.

Each ingress will be for a different domain (`argocd.example.com` and `grpc.argocd.example.com`). This requires that the Ingress resources use different TLS `secretName`s to avoid unexpected behavior.

HTTP/HTTPS Ingress:
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: argocd-server-http-ingress
  namespace: argocd
  annotations:
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
    nginx.ingress.kubernetes.io/backend-protocol: "HTTP"
spec:
  ingressClassName: nginx
  rules:
  - http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: argocd-server
            port:
              name: http
    host: argocd.example.com
  tls:
  - hosts:
    - argocd.example.com
    secretName: argocd-ingress-http
```

gRPC Ingress:
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: argocd-server-grpc-ingress
  namespace: argocd
  annotations:
    nginx.ingress.kubernetes.io/backend-protocol: "GRPC"
spec:
  ingressClassName: nginx
  rules:
  - http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: argocd-server
            port:
              name: https
    host: grpc.argocd.example.com
  tls:
  - hosts:
    - grpc.argocd.example.com
    secretName: argocd-ingress-grpc
```

The API server should then be run with TLS disabled. Edit the `argocd-server` deployment to add the
`--insecure` flag to the argocd-server command, or simply set `server.insecure: "true"` in the `argocd-cmd-params-cm` ConfigMap [as described here](server-commands/additional-configuration-method.md).

The obvious disadvantage to this approach is that this technique requires two separate hostnames for
the API server -- one for gRPC and the other for HTTP/HTTPS. However it allows TLS termination to
happen at the ingress controller.


## [Traefik (v2.2)](https://docs.traefik.io/)

Traefik can be used as an edge router and provide [TLS](https://docs.traefik.io/user-guides/grpc/) termination within the same deployment.

It currently has an advantage over NGINX in that it can terminate both TCP and HTTP connections _on the same port_ meaning you do not require multiple hosts or paths.

The API server should be run with TLS disabled. Edit the `argocd-server` deployment to add the `--insecure` flag to the argocd-server command or set `server.insecure: "true"` in the `argocd-cmd-params-cm` ConfigMap [as described here](server-commands/additional-configuration-method.md).

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
```

## AWS Application Load Balancers (ALBs) And Classic ELB (HTTP Mode)
AWS ALBs can be used as an L7 Load Balancer for both UI and gRPC traffic, whereas Classic ELBs and NLBs can be used as L4 Load Balancers for both.

When using an ALB, you'll want to create a second service for argocd-server. This is necessary because we need to tell the ALB to send the GRPC traffic to a different target group then the UI traffic, since the backend protocol is HTTP2 instead of HTTP1.

```yaml
apiVersion: v1
kind: Service
metadata:
  annotations:
    alb.ingress.kubernetes.io/backend-protocol-version: HTTP2 #This tells AWS to send traffic from the ALB using HTTP2. Can use GRPC as well if you want to leverage GRPC specific features
  labels:
    app: argogrpc
  name: argogrpc
  namespace: argocd
spec:
  ports:
  - name: "443"
    port: 443
    protocol: TCP
    targetPort: 8080
  selector:
    app.kubernetes.io/name: argocd-server
  sessionAffinity: None
  type: NodePort
```

Once we create this service, we can configure the Ingress to conditionally route all `application/grpc` traffic to the new HTTP2 backend, using the `alb.ingress.kubernetes.io/conditions` annotation, as seen below. Note: The value after the . in the condition annotation _must_ be the same name as the service that you want traffic to route to - and will be applied on any path with a matching serviceName.

```yaml
  apiVersion: networking.k8s.io/v1
  kind: Ingress
  metadata:
    annotations:
      alb.ingress.kubernetes.io/backend-protocol: HTTPS
      # Use this annotation (which must match a service name) to route traffic to HTTP2 backends.
      alb.ingress.kubernetes.io/conditions.argogrpc: |
        [{"field":"http-header","httpHeaderConfig":{"httpHeaderName": "Content-Type", "values":["application/grpc"]}}]
      alb.ingress.kubernetes.io/listen-ports: '[{"HTTPS":443}]'
    name: argocd
    namespace: argocd
  spec:
    rules:
    - host: argocd.argoproj.io
      http:
        paths:
        - path: /
          backend:
            service:
              name: argogrpc
              port:
                number: 443
          pathType: Prefix
        - path: /
          backend:
            service:
              name: argocd-server
              port:
                number: 443
          pathType: Prefix
    tls:
    - hosts:
      - argocd.argoproj.io
```

## [Istio](https://www.istio.io)
You can put Argo CD behind Istio using following configurations. Here we will achive both serving Argo CD behind istio and using subpath on Istio

First we need to make sure that we can run Argo CD with subpath (ie /argocd). For this we have used install.yaml from argocd project as is

```bash
curl -kLs -o install.yaml https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

save following file as kustomization.yml

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- ./install.yaml

patches:
- path: ./patch.yml
``` 

And following lines as patch.yml

```yaml
# Use --insecure so Ingress can send traffic with HTTP
# --bashref /argocd is the subpath like https://IP/argocd
# env was added because of https://github.com/argoproj/argo-cd/issues/3572 error
---
apiVersion: apps/v1
kind: Deployment
metadata:
 name: argocd-server
spec:
 template:
   spec:
     containers:
     - args:
       - /usr/local/bin/argocd-server
       - --staticassets
       - /shared/app
       - --redis
       - argocd-redis-ha-haproxy:6379
       - --insecure
       - --basehref
       - /argocd
       - --rootpath
       - /argocd
       name: argocd-server
       env:
       - name: ARGOCD_MAX_CONCURRENT_LOGIN_REQUESTS_COUNT
         value: "0"
```

After that install Argo CD  (there should be only 3 yml file defined above in current directory )

```bash
kubectl apply -k ./ -n argocd --wait=true
```

Be sure you create secret for Isito ( in our case secretname is argocd-server-tls on argocd Namespace). After that we create Istio Resources

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: Gateway
metadata:
  name: argocd-gateway
  namespace: argocd
spec:
  selector:
    istio: ingressgateway
  servers:
  - port:
      number: 80
      name: http
      protocol: HTTP
    hosts:
    - "*"
    tls:
     httpsRedirect: true
  - port:
      number: 443
      name: https
      protocol: HTTPS
    hosts:
    - "*"
    tls:
      credentialName: argocd-server-tls
      maxProtocolVersion: TLSV1_3
      minProtocolVersion: TLSV1_2
      mode: SIMPLE
      cipherSuites:
        - ECDHE-ECDSA-AES128-GCM-SHA256
        - ECDHE-RSA-AES128-GCM-SHA256
        - ECDHE-ECDSA-AES128-SHA
        - AES128-GCM-SHA256
        - AES128-SHA
        - ECDHE-ECDSA-AES256-GCM-SHA384
        - ECDHE-RSA-AES256-GCM-SHA384
        - ECDHE-ECDSA-AES256-SHA
        - AES256-GCM-SHA384
        - AES256-SHA
---
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: argocd-virtualservice
  namespace: argocd
spec:
  hosts:
  - "*"
  gateways:
  - argocd-gateway
  http:
  - match:
    - uri:
        prefix: /argocd
    route:
    - destination:
        host: argocd-server
        port:
          number: 80
```

And now we can browse http://{{ IP }}/argocd (it will be rewritten to https://{{ IP }}/argocd


## Google Cloud load balancers with Kubernetes Ingress

You can make use of the integration of GKE with Google Cloud to deploy Load Balancers using just Kubernetes objects.

For this we will need these five objects:
- A Service
- A BackendConfig
- A FrontendConfig
- A secret with your SSL certificate
- An Ingress for GKE

If you need detail for all the options available for these Google integrations, you can check the [Google docs on configuring Ingress features](https://cloud.google.com/kubernetes-engine/docs/how-to/ingress-features)

### Disable internal TLS

First, to avoid internal redirection loops from HTTP to HTTPS, the API server should be run with TLS disabled.

Edit the `--insecure` flag in the `argocd-server` command of the argocd-server deployment, or simply set `server.insecure: "true"` in the `argocd-cmd-params-cm` ConfigMap [as described here](server-commands/additional-configuration-method.md).

### Creating a service

Now you need an externally accessible service. This is practically the same as the internal service Argo CD has, but with Google Cloud annotations. Note that this service is annotated to use a [Network Endpoint Group](https://cloud.google.com/load-balancing/docs/negs) (NEG) to allow your load balancer to send traffic directly to your pods without using kube-proxy, so remove the `neg` annotation it that's not what you want.

The service:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: argocd-server
  namespace: argocd
  annotations:
    cloud.google.com/neg: '{"ingress": true}'
    cloud.google.com/backend-config: '{"ports": {"http":"argocd-backend-config"}}'
spec:
  type: ClusterIP
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: 8080
  selector:
    app.kubernetes.io/name: argocd-server
```

### Creating a BackendConfig

See that previous service referencing a backend config called `argocd-backend-config`? So lets deploy it using this yaml:

```yaml
apiVersion: cloud.google.com/v1
kind: BackendConfig
metadata:
  name: argocd-backend-config
  namespace: argocd
spec:
  healthCheck:
    checkIntervalSec: 30
    timeoutSec: 5
    healthyThreshold: 1
    unhealthyThreshold: 2
    type: HTTP
    requestPath: /healthz
    port: 8080
```

It uses the same health check as the pods.

### Creating a FrontendConfig

Now we can deploy a frontend config with an HTTP to HTTPS redirect:

```yaml
apiVersion: networking.gke.io/v1beta1
kind: FrontendConfig
metadata:
  name: argocd-frontend-config
  namespace: argocd
spec:
  redirectToHttps:
    enabled: true
```

---
!!! note

    The next two steps (the certificate secret and the Ingress) are described supposing that you manage the certificate yourself, and you have the certificate and key files for it. In the case that your certificate is Google-managed, fix the next two steps using the [guide to use a Google-managed SSL certificate](https://cloud.google.com/kubernetes-engine/docs/how-to/managed-certs#creating_an_ingress_with_a_google-managed_certificate).

---

### Creating a certificate secret

We need now to create a secret with the SSL certificate we want in our load balancer. It's as easy as executing this command on the path you have your certificate keys stored:

```
kubectl -n argocd create secret tls secret-yourdomain-com \
  --cert cert-file.crt --key key-file.key
```

### Creating an Ingress

And finally, to top it all, our Ingress. Note the reference to our frontend config, the service, and to the certificate secret.

---
!!! note

   GKE clusters running versions earlier than `1.21.3-gke.1600`, [the only supported value for the pathType field](https://cloud.google.com/kubernetes-engine/docs/how-to/load-balance-ingress#creating_an_ingress) is `ImplementationSpecific`. So you must check your GKE cluster's version. You need to use different YAML depending on the version.

---

If you use the version earlier than `1.21.3-gke.1600`, you should use the following Ingress resource:
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: argocd
  namespace: argocd
  annotations:
    networking.gke.io/v1beta1.FrontendConfig: argocd-frontend-config
spec:
  tls:
    - secretName: secret-example-com
  rules:
    - host: argocd.example.com
      http:
        paths:
        - pathType: ImplementationSpecific
          path: "/*"   # "*" is needed. Without this, the UI Javascript and CSS will not load properly
          backend:
            service:
              name: argocd-server
              port:
                number: 80
```

If you use the version `1.21.3-gke.1600` or later, you should use the following Ingress resource:
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: argocd
  namespace: argocd
  annotations:
    networking.gke.io/v1beta1.FrontendConfig: argocd-frontend-config
spec:
  tls:
    - secretName: secret-example-com
  rules:
    - host: argocd.example.com
      http:
        paths:
        - pathType: Prefix
          path: "/"
          backend:
            service:
              name: argocd-server
              port:
                number: 80
```

As you may know already, it can take some minutes to deploy the load balancer and become ready to accept connections. Once it's ready, get the public IP address for your Load Balancer, go to your DNS server (Google or third party) and point your domain or subdomain (i.e. argocd.example.com) to that IP address.

You can get that IP address describing the Ingress object like this:

```
kubectl -n argocd describe ingresses argocd | grep Address
```

Once the DNS change is propagated, you're ready to use Argo with your Google Cloud Load Balancer

## Authenticating through multiple layers of authenticating reverse proxies

Argo CD endpoints may be protected by one or more reverse proxies layers, in that case, you can provide additional headers through the `argocd` CLI `--header` parameter to authenticate through those layers.

```shell
$ argocd login <host>:<port> --header 'x-token1:foo' --header 'x-token2:bar' # can be repeated multiple times
$ argocd login <host>:<port> --header 'x-token1:foo,x-token2:bar' # headers can also be comma separated
```
## ArgoCD Server and UI Root Path (v1.5.3)

Argo CD server and UI can be configured to be available under a non-root path (e.g. `/argo-cd`).
To do this, add the `--rootpath` flag into the `argocd-server` deployment command:

```yaml
spec:
  template:
    spec:
      name: argocd-server
      containers:
      - command:
        - /argocd-server
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
