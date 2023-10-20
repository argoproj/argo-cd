# Prerequisites
* K8s cluster with kubectl configured
* Argo CD with remote managed clusters

# Deploy benchmark environment
1. There are two configurable parameters to building the benchmark environment:
* appdist (random or equal): determines how the applications would be distributed to the target clusters. Currently the environment build will utilize all clusters defined in Argo CD/
* numapps: how many applications you want to generate.

2. Example command:
```
./dist/argocd-benchmark buildenv --appdist random --numapps 5000
```

# Perform benchmark
1. Choose from the avaliable benchmark tests. Currently there is only one: synctest.
* synctest: Pushes a change to the monorepo that all applications are connected to. Will trigger a sync on all applications and return the time it took to perform the sync.

2. Example command:
```
./dist/argocd-benchmark benchmark --testtype synctest
```