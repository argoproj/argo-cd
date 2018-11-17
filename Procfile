controller: go run ./cmd/argocd-application-controller/main.go
api-server: go run ./cmd/argocd-server/main.go --insecure --disable-auth --dex-server http://localhost:5556 --repo-server localhost:8081 --app-controller-server localhost:8083
repo-server: go run ./cmd/argocd-repo-server/main.go --loglevel debug
dex: sh -c "go run ./cmd/argocd-util/main.go gendexcfg -o `pwd`/dist/dex.yaml && docker run --rm -p 5556:5556 -v `pwd`/dist/dex.yaml:/dex.yaml quay.io/dexidp/dex:v2.12.0 serve /dex.yaml"
