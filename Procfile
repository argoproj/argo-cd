controller: ./hack/run/application-controller.sh
api-server: ./hack/run/api-server.sh
dex: ./hack/run/dex.sh
redis: ./hack/run/redis.sh
repo-server: ./hack/run/repo-server.sh
cmp-server: ./hack/run/cmp-server.sh
ui: sh -c 'cd ui && ${ARGOCD_E2E_YARN_CMD:-yarn} start'
git-server: test/fixture/testrepos/start-git.sh
helm-registry: test/fixture/testrepos/start-helm-registry.sh
dev-mounter: ./hack/run/dev-mounter.sh
applicationset-controller: ./hack/run/applicationset-controller.sh
notification: ./hack/run/notification-controller.sh
