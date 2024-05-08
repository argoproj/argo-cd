# API Docs

You can find the Swagger docs by setting the path to `/swagger-ui` in your Argo CD UI. E.g. [http://localhost:8080/swagger-ui](http://localhost:8080/swagger-ui).

## Authorization

You'll need to authorize your API using a bearer token. To get a token:

```bash
$ curl $ARGOCD_SERVER/api/v1/session -d $'{"username":"admin","password":"password"}'
{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpYXQiOjE1Njc4MTIzODcsImlzcyI6ImFyZ29jZCIsIm5iZiI6MTU2NzgxMjM4Nywic3ViIjoiYWRtaW4ifQ.ejyTgFxLhuY9mOBtKhcnvobg3QZXJ4_RusN_KIdVwao"} 
```

Then pass using the HTTP `Authorization` header, prefixing with `Bearer `:

```bash
$ curl $ARGOCD_SERVER/api/v1/applications -H "Authorization: Bearer $ARGOCD_TOKEN" 
{"metadata":{"selfLink":"/apis/argoproj.io/v1alpha1/namespaces/argocd/applications","resourceVersion":"37755"},"items":...}
```

## Services

### Applications API

#### How to Avoid 403 Errors for Missing Applications

All endpoints of the Applications API accept an optional `project` query string parameter. If the parameter 
is specified, and the specified Application does not exist, the API will return a `404` error.

Additionally, if the `project` query string parameter is specified and the Application exists but is not in 
the given `project`, the API will return a `403` error. This is to prevent leaking information about the 
existence of Applications to users who do not have access to them.