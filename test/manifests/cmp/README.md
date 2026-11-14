This folder contains an Argo CD configuration file to allow
testing CMP plugins locally. The Kustomize project will:

- Install Argo CD in the current k8s context
- Patch repo server configuring a test CMP plugin
- Install an application that can be used to interact with the CMP plugin

To install Argo CD with this Kustomize project run the following
command:

`kustomize build ./test/manifests/cmp | sed 's/imagePullPolicy: Always/imagePullPolicy: Never/g' | kubectl apply -f -`

In Argo CD UI login with user/pass: admin/password

An application with name `cmp-sidecar` should be available for testing.
