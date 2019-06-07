controller: sh -c "FORCE_LOG_COLORS=1 ARGOCD_FAKE_IN_CLUSTER=true go run ./cmd/argocd-application-controller/main.go --loglevel debug --redis localhost:6379 --repo-server localhost:8081"
api-server: sh -c "FORCE_LOG_COLORS=1 ARGOCD_FAKE_IN_CLUSTER=true go run ./cmd/argocd-server/main.go --loglevel debug --redis localhost:6379 --disable-auth --insecure --dex-server http://localhost:5556 --repo-server localhost:8081 --staticassets ui/dist/app"
dex: sh -c "go run ./cmd/argocd-util/main.go gendexcfg -o `pwd`/dist/dex.yaml && docker run --rm -p 5556:5556 -v `pwd`/dist/dex.yaml:/dex.yaml quay.io/dexidp/dex:v2.14.0 serve /dex.yaml"
redis: docker run --rm --name argocd-redis -i -p 6379:6379 redis:5.0.3-alpine --save "" --appendonly no
repo-server: sh -c "FORCE_LOG_COLORS=1 go run ./cmd/argocd-repo-server/main.go --loglevel debug --redis localhost:6379"
ui: sh -c 'cd ui && yarn start'