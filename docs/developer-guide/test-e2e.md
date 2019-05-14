# E2E Tests

The directory contains E2E tests and test applications. The test assume that Argo CD services are installed into `argocd-e2e` namespace or cluster in current context. One throw-away
namespace `argocd-e2e***` is created prior to tests execute. The throw-away namespace is used as a target namespace for test applications.

The `test/e2e/testdata` directory contains various Argo CD applications. Before test execution directory is copies into `/tmp/argocd-e2e***` temp directory and used in tests as a
Git repository via file url: `file:///tmp/argocd-e2e***`.

## Running Tests Locally

1. Start the e2e version `make start-e2e` 
1. Run the tests: `make test-e2e`

You can observe the tests by using the UI [http://localhost:8080/applications](http://localhost:8080/applications).

## CI Set-up

The tests are executed by Argo Workflow defined at `.argo-ci/ci.yaml`. CI job The builds an Argo CD image, deploy argo cd components into throw-away kubernetes cluster provisioned
using k3s and run e2e tests against it.

## Test Isolation

Some effort has been made to balance test isolation with speed. Tests are isolated as follows as each test gets:
 
* A random 5 character ID.
* A unique Git repository containing the `testdata` in `/tmp/argocd-e2e/${id}`.
* A namespace `argocd-e2e-ns-${id}`.
* An primary name for the app `argocd-e2e-${id}`.

## Troubleshooting

**Tests fails to delete `argocd-e2e-ns-*` namespaces.**

This maybe due to the metrics server, run this:

```bash
kubectl api-resources 
```

If it exits with status code 1, run:

```bash
kubectl delete apiservice v1beta1.metrics.k8s.io
```

Remove `/spec/finalizers` from the namespace

```bash
kubectl edit ns argocd-e2e-ns-*
```