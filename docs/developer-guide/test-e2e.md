# E2E Tests

The directory contains E2E tests and test applications. The test assume that Argo CD services are installed into `argocd-e2e` namespace or cluster in current context. One throw-away
namespace `argocd-e2e***` is created prior to tests execute. The throw-away namespace is used as a target namespace for test applications.

The `test/e2e/testdata` directory contains various Argo CD applications. Before test execution directory is copies into `/tmp/argocd-e2e***` temp directory and used in tests as a
Git repository via file url: `file:///tmp/argocd-e2e***`.

## Running Tests Locally

1. Create namespace: `kubectl create ns argocd-e2e` 
1. Apply manifests: `kustomize build test/manifests/base | kubectl apply -n argocd-e2e -f -`
1. Change namespace to `argocd-e2e`: `kubens argocd-e2e`
1. Start services: `make start`
1. Build CLI: `make cli`
1. Run tests: `make test-e2e`

You can observe the tests by using the UI [http://localhost:8080/applications](http://localhost:8080/applications).

## CI Set-up

The tests are executed by Argo Workflow defined at `.argo-ci/ci.yaml`. CI job The builds an Argo CD image, deploy argo cd components into throw-away kubernetes cluster provisioned
using k3s and run e2e tests against it.
