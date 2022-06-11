# API Docs

You can find the Swagger docs by setting the path to `/swagger-ui` in your Argo CD UI's. E.g. [http://localhost:8080/swagger-ui](http://localhost:8080/swagger-ui).

## Authorization

You'll need to authorize your API using a bearer token. To get a token:

```bash
$ curl $ARGOCD_SERVER/api/v1/session -d $'{"username":"admin","password":"password"}'
{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpYXQiOjE1Njc4MTIzODcsImlzcyI6ImFyZ29jZCIsIm5iZiI6MTU2NzgxMjM4Nywic3ViIjoiYWRtaW4ifQ.ejyTgFxLhuY9mOBtKhcnvobg3QZXJ4_RusN_KIdVwao"} 
```

> <=v1.2

Then pass using the HTTP `SetCookie` header, prefixing with `argocd.token`:

```bash
$ curl $ARGOCD_SERVER/api/v1/applications --cookie "argocd.token=$ARGOCD_TOKEN" 
{"metadata":{"selfLink":"/apis/argoproj.io/v1alpha1/namespaces/argocd/applications","resourceVersion":"37755"},"items":...}
```

> v1.3

Then pass using the HTTP `Authorization` header, prefixing with `Bearer `:

```bash
$ curl $ARGOCD_SERVER/api/v1/applications -H "Authorization: Bearer $ARGOCD_TOKEN" 
{"metadata":{"selfLink":"/apis/argoproj.io/v1alpha1/namespaces/argocd/applications","resourceVersion":"37755"},"items":...}
```
 
