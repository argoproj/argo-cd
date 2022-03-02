# End-to-end tests against a real cluster

Using the tools in this directory, you can run the End-to-End testsuite against
a real Argo CD workload, that is deployed to a K8s cluster, instead of running
it against a locally running Argo CD.

Since e2e tests are destructive, do **not** run it against an installation that
you depend on.

## Preparations

### Install the Argo CD you want to test

It is recommended to install in the `argocd-e2e` namespace:

```shell
kubectl create ns argocd-e2e
kubectl -n argocd-e2e apply -f <your Argo CD installation manifests>
```

If you're going to install Argo CD using either Argo CD Operator or OpenShift GitOps Operator, you can use this manifest to install the Argo CD instance into your namespace:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: argocd-test
  namespace: argocd-e2e
spec:
  server:
    route:
      enabled: true
```

### Give the Argo CD the appropriate RBAC permissions

```shell
# If you installed to a different namespace, set accordingly
export NAMESPACE=argocd-e2e
# If you installed Argo CD via Operator, set accordingly
# export ARGOCD_E2E_NAME_PREFIX=argocd-cluster
./test/remote/generate-permissions.sh | kubectl apply -f -
```

### Build the repository container image

You will need to build & publish a container image that will hold the required
testing repositories.

This container image will be named `argocd-e2e-cluster`, so you will need to
setup a corresponding repository for it in your registry as well.

To build it, run the following. Note that kustomize is required:

```shell
cd test/remote
export IMAGE_NAMESPACE=quay.io/{YOUR USERNAME HERE}
# builds & tags the image
make image
# pushes the image to your repository
make image-push
# build the manifests & store them at temp location
make manifests > /tmp/e2e-repositories.yaml
```

If you do not have kustomize installed, you need to manually edit the manifests
at `test/remote/manifests/e2e_repositories.yaml` to point to the correct image.

If you get `make: realpath: Command not found`, install coreutils.

### Deploy the test container and additional permissions

**Note:** The test container requires to be run in privileged mode for now, due
to some processes running as root (this may change some day...).

On OpenShift, you will likely need to allow privileged operations:

```shell
oc -n argocd-e2e adm policy add-scc-to-user privileged -z default
```

Then, apply the manifests for the E2E repositories workload:

```shell
kubectl -n argocd-e2e apply -f /tmp/e2e-repositories.yaml
```

Verify that the deployment was successful:

```shell
kubectl -n argocd-e2e rollout status deployment argocd-e2e-cluster
```

## Start the tests

### Port-forward the repository

In another shell, port forward the repository to your local machine:

```shell
kubectl -n argocd-e2e port-forward service/argocd-e2e-server 9081:9081
```

### On local cluster (e.g. K3s, microk8s, minishift)

Set the server endpoint of the Argo CD API. If you are running on the same host
as the cluster, or the cluster IPs are routed to your host, you can use the
following:

```shell
export ARGOCD_SERVER=$(kubectl -n argocd-e2e get svc argocd-server -o jsonpath='{.spec.clusterIP}')
```

Set the admin password to use:

```shell
export ARGOCD_E2E_ADMIN_PASSWORD=$(kubectl -n argocd-e2e get secrets argocd-initial-admin-secret -o jsonpath='{.data.password}'|base64 -d)
```

Run the tests

```shell
./test/remote/run-e2e-remote.sh make test-local
```

### On remote non-OpenShift cluster

In another shell, do a port-forward to the API server's service:

```shell
kubectl -n argocd-e2e port-forward svc/argocd-server 443:4443
```

Set Argo CD Server endport:

```shell
export ARGOCD_SERVER=127.0.0.1:4443
```

Set the admin password to use:

```shell
export ARGOCD_E2E_ADMIN_PASSWORD=$(kubectl get secrets argocd-initial-admin-secret -o jsonpath='{.data.password}'|base64 -d)
```

Run the tests

```shell
./test/remote/run-e2e-remote.sh make test-local
```

### On remote OpenShift cluster

You should first scale down Argo CD Operator, since it will revert changes made
during the tests instantly:

```shell
oc -n openshift-operators scale deployment --replicas=0 argocd-operator
```

Set the endpoint by using the route created by the operator:

```shell
export ARGOCD_SERVER=$(oc -n argocd-e2e get routes argocd-test-server -o jsonpath='{.spec.host}')
```

Set the admin password, created by the operator:

```shell
export ARGOCD_E2E_ADMIN_PASSWORD=$(oc -n argocd-e2e get secrets argocd-test-cluster -o jsonpath='{.data.admin\.password}' | base64 -d)
```

Set the name of the Argo CD instance as installed by the operator (below example assumes operand name of `argocd-test`):

```shell
export ARGOCD_E2E_NAME_PREFIX=argocd-test
```

Run the tests with currently known failing tests disabled:

```shell
./test/remote/run-e2e-remote.sh make test-local ARGOCD_E2E_SKIP_OPENSHIFT=true
```

## Running single or multiple tests isolated

This should be run in the same shell where you set `ARGOCD_SERVER` and `ARGOCD_E2E_ADMIN_PASSWORD` variables

1. Run the `go test` command through the wrapper, e.g. to run a test named `MyTestName`:

   ```
   $ ./test/remote/run-e2e-remote.sh go test -v github.com/argoproj/argo-cd/test/e2e -run ^MyTestName$^
   ```


## Further configuration

Some environment variables can control the behavior of the tests:

* `ARGOCD_SERVER` - the remote endpoint of the Argo CD API server to use for tests
* `ARGOCD_E2E_ADMIN_PASSWORD` - the admin user's password to use
* `ARGOCD_E2E_TEST_TIMEOUT` - timeout for the complete test suite, specified as duration (e.g. `2h` or `1h30m`)
* `ARGOCD_E2E_DEFAULT_TIMEOUT` - timeout in seconds for each context operation (e.g. sync). Default `30`.
* `ARGOCD_E2E_NAMESPACE` - the namespace where Argo CD is running in for the tests
* `ARGOCD_E2E_NAME_PREFIX` - if your Argo CD installation has a name prefix (e.g. installed by the Operator), specify it here

Furthermore, you can skip various classes of tests by setting the following to true:

```shell
# If you disabled GPG feature, set to true to skip related tests
export ARGOCD_E2E_SKIP_GPG=${ARGOCD_E2E_SKIP_GPG:-false}
# Some tests do not work OOTB with OpenShift
export ARGOCD_E2E_SKIP_OPENSHIFT=${ARGOCD_E2E_SKIP_OPENSHIFT:-false}
# Skip all Helm tests
export ARGOCD_E2E_SKIP_HELM=${ARGOCD_E2E_SKIP_HELM:-false}
# Skip only Helm v2 related tests
export ARGOCD_E2E_SKIP_HELM2=${ARGOCD_E2E_SKIP_HELM2:-false}
```

## Recording tests that ran successfully and restart at point of fail

Sometimes, due to a hiccup or timeout on the remote cluster, a test may fail to run without specific reason. This can be time consuming when tests
are being run again, since every test will be executed again. You can record the tests that were run & successful and pick up where it failed.
For this purpose, set the `ARGOCD_E2E_RECORD` variable to point to a file where tests will be recorded:

   ```
   $ ./test/remote/run-e2e-remote.sh make test-e2e-local ARGOCD_E2E_RECORD=/tmp/mytests
   ```

If the tests fail, just re-run above command. All tests that have been previously run will be skipped, and testing will continue with the next test.

## Tear down

1. Remove argocd-e2e namespace

   ```
   $ kubectl delete ns argocd-e2e
   ```


## Troubleshooting

* On message:
  
  ```
  time="2021-03-23T09:52:53Z" level=fatal msg="`git push origin master -f` failed exit status 128: fatal: unable to access 'http://127.0.0.1:9081/argo-e2e/testdata.git/': Empty reply from server"
  ```

  Your port-forward is probably not setup correctly or broke (e.g. due to pod restart)

* Make sure `argocd-e2e-cluster` pod is running. If you get a CrashLoopBackoff, ensure that you enabled elevated privs as shown above

* Sometimes, you may run into a timeout especially if the cluster is very busy. In this case, you have to restart the tests. See test recording above.
