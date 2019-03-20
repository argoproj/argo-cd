controller: go run ./cmd/argocd-application-controller/main.go --loglevel debug --redis localhost:6379 --repo-server localhost:8081
api-server: go run ./cmd/argocd-server/main.go --loglevel debug --redis localhost:6379 --disable-auth --insecure --dex-server http://localhost:5556 --repo-server localhost:8081 --staticassets ../argo-cd-ui/dist/app
repo-server: go run ./cmd/argocd-repo-server/main.go --loglevel debug --redis localhost:6379
dex: sh -c "go run ./cmd/argocd-util/main.go gendexcfg -o `pwd`/dist/dex.yaml && docker run --rm -p 5556:5556 -v `pwd`/dist/dex.yaml:/dex.yaml quay.io/dexidp/dex:v2.14.0 serve /dex.yaml"
redis: docker run --rm -i -p 6379:6379 redis:5.0.3-alpine --save "" --appendonly no
ui: sh -c 'cd ../argo-cd-ui && yarn start'