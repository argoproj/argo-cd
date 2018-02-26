The ksonnet test-app was generated using the following commands
```
ks init test-app --api-spec=version:v1.8.0
cd test-app
# edit app.yaml to use test-namespace and test-env instead of default
ks generate deployed-service demo --image=gcr.io/kuar-demo/kuard-amd64:1 --replicas=2 --type=ClusterIP
```