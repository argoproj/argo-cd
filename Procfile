controller: sh -c "FORCE_LOG_COLORS=1 ARGOCD_FAKE_IN_CLUSTER=true go run ./cmd/argocd-application-controller/main.go --loglevel debug --redis localhost:${ARGOCD_E2E_REDIS_PORT:-6379} --repo-server localhost:${ARGOCD_E2E_REPOSERVER_PORT:-8081}"
api-server: sh -c "FORCE_LOG_COLORS=1 ARGOCD_FAKE_IN_CLUSTER=true go run ./cmd/argocd-server/main.go --loglevel debug --redis localhost:${ARGOCD_E2E_REDIS_PORT:-6379} --disable-auth --insecure --dex-server http://localhost:${ARGOCD_E2E_DEX_PORT:-5556} --repo-server localhost:${ARGOCD_E2E_REPOSERVER_PORT:-8081} --port ${ARGOCD_E2E_APISERVER_PORT:-8080} --staticassets ui/dist/app"
dex: sh -c "go run ./cmd/argocd-util/main.go gendexcfg -o `pwd`/dist/dex.yaml && docker run --rm -p ${ARGOCD_E2E_DEX_PORT:-5556}:${ARGOCD_E2E_DEX_PORT:-5556} -v `pwd`/dist/dex.yaml:/dex.yaml quay.io/dexidp/dex:v2.14.0 serve /dex.yaml"
redis: docker run --rm --name argocd-redis -i -p ${ARGOCD_E2E_REDIS_PORT:-6379}:${ARGOCD_E2E_REDIS_PORT:-6379} redis:5.0.3-alpine --save "" --appendonly no --port ${ARGOCD_E2E_REDIS_PORT:-6379}
repo-server: sh -c "FORCE_LOG_COLORS=1 ARGOCD_FAKE_IN_CLUSTER=true go run ./cmd/argocd-repo-server/main.go --loglevel debug --port ${ARGOCD_E2E_REPOSERVER_PORT:-8081} --redis localhost:${ARGOCD_E2E_REDIS_PORT:-6379}"
ui: sh -c 'cd ui && ${ARGOCD_E2E_YARN_CMD:-yarn} start'
git-server: test/fixture/testrepos/start-git.sh
