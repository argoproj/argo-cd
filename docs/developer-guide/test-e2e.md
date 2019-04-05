E2E tests
=============

The directory contains E2E tests and test applications. Tests assume that Argo CD services are installed into `argocd-e2e` namespace or cluster in current context. One throw-away
namespace `argocd-e2e***` is created prior to tests execute. The throw-away namespace is used as a target namespace for test applications.

The `test/e2e/testdata` directory contains various Argo CD applications. Before test execution directory is copies into `/tmp/argocd-e2e***` temp directory and used in tests as a
Git repository via file url: `file:///tmp/argocd-e2e***`.

Use the following steps to run tests locally:

1. (Do it once) Create namespace `argocd-e2e` and apply base manifests: `kubectl create ns -n argocd-e2e && kustomize build test/manifests/base | kubectl apply -n argocd-e2e -f -`
1. Change kubectl context namespace to `argocd-e2e` and start services using `goreman start`
1. Keep Argo CD services running and run tests using `make test-e2e`

The tests are executed by Argo Workflow defined at `.argo-ci/ci.yaml`. CI job The build argo cd image, deploy argo cd components into throw-away kubernetes cluster provisioned
using k3s and run e2e tests against it.
