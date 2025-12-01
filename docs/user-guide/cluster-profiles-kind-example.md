# Using Cluster Profiles with Kind Clusters

This guide demonstrates how to use Cluster Profiles to connect a spoke cluster to an Argo CD instance running in a hub cluster.

## Prerequisites

- Docker, Kind, Kubectl

## 1. Create Hub and Spoke Clusters

Create two `kind` clusters:
```bash
kind create cluster --name hub
kind create cluster --name spoke
```

## 2. Install Argo CD

Install Argo CD in `hub`:

```bash
kubectl config use-context kind-hub
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=argocd-server -n argocd --timeout=300s
```

NOTE: if doing local development in a fork of the argo-cd repo, build the local image:
```bash
export IMAGE_NAMESPACE=quay.io/argoproj
export IMAGE_TAG=my-dev-v1
export DOCKER_PUSH=false
make image
kind load docker-image ${IMAGE_NAMESPACE}/argocd:${IMAGE_TAG} --name hub
make manifests
kubectl apply -n argocd -f manifests/install.yaml
```

Create argocd manager service account in `spoke`:
```bash
kubectl config use-context kind-spoke
kubectl apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: argocd-manager
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: argocd-manager-role
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: argocd-manager
  namespace: kube-system
---
apiVersion: v1
kind: Secret
metadata:
  name: argocd-manager-token
  namespace: kube-system
  annotations:
    kubernetes.io/service-account.name: argocd-manager
type: kubernetes.io/service-account-token
EOF
```

Create namespace for future application:
```bash
kubectl create namespace guestbook
```

## 3. Get spoke credentials

```bash
SPOKE_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' spoke-control-plane)
SPOKE_CA=$(kubectl --context kind-spoke config view --raw --minify --flatten -o jsonpath='{.clusters[0].cluster.certificate-authority-data}')
SPOKE_TOKEN=$(kubectl --context kind-spoke -n kube-system get secret argocd-manager-token -o jsonpath='{.data.token}' | base64 -d)
```

## 4. Create access providers file in hub

Create access providers secret:
```bash
kubectl config use-context kind-hub
kubectl create secret -n argocd generic cp-creds-secret \
  -n argocd \
  --from-file=cp-creds.json=/dev/stdin <<EOF
{
  "providers": [
    {
      "name": "hub-provider",
      "execConfig": {
        "command": "/usr/local/bin/custom-auth/get-token.sh",
        "apiVersion": "client.authentication.k8s.io/v1beta1"
      },
      "tlsClientConfig": {
        "caData": "${SPOKE_CA}"
      }
    }
  ]
}
EOF
```

This will be read by the Cluster Profile syncer and used to generate an Argo CD cluster secret.

## 5. Create Cluster Profile in hub

Create Cluster Profile:
```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-inventory-api/main/config/crd/bases/multicluster.x-k8s.io_clusterprofiles.yaml
kubectl apply -f - <<EOF
apiVersion: "multicluster.x-k8s.io/v1alpha1"
kind: ClusterProfile
metadata:
  name: spoke-cluster
  namespace: argocd
spec:
  clusterManager:
    name: manual
  displayName: "Spoke Cluster"
EOF
```

Normally, a controller would automatically update the `status`. For this example, patch it in manually.

Add access provider (the provider's `name` same as in the access providers file):
```bash
kubectl patch clusterprofile spoke-cluster -n argocd --subresource=status --type=merge -p '{"status":{"accessProviders":[{"name":"hub-provider","cluster":{"server":"https://'"${SPOKE_IP}"':6443", "certificate-authority-data": "'"${SPOKE_CA}"'"}}]}}'
```

Mount the access providers file into application controller:
```bash
kubectl -n argocd patch deploy/argocd-applicationset-controller --type strategic --patch '
spec:
  template:
    spec:
      volumes:
        - name: cp-creds-vol
          secret:
            secretName: cp-creds-secret
      containers:
        - name: argocd-applicationset-controller
          volumeMounts:
            - name: cp-creds-vol
              mountPath: /app/cp-creds
          args:
            - "/usr/local/bin/argocd-applicationset-controller"
            - "--cluster-profile-providers-file=/app/cp-creds/cp-creds.json"
'
```

Setting a value for `--cluster-profile-providers-file` will enable the Cluster Profile syncer in the applicationset controller.

## 6. Create a plugin to use credentials

Create a simple auth plugin:
```bash
kubectl create configmap argocd-custom-auth-plugin -n argocd --from-literal=get-token.sh='#!/bin/sh
cat <<EOF
{
  "apiVersion": "client.authentication.k8s.io/v1beta1",
  "kind": "ExecCredential",
  "status": {
    "token": "'"$SPOKE_TOKEN"'"
  }
}
EOF
'
```

Mount the auth plugin into the application controller:
```bash
kubectl -n argocd patch sts/argocd-application-controller --type strategic --patch '
spec:
  template:
    spec:
      volumes:
        - name: auth-script
          configMap:
            name: argocd-custom-auth-plugin
            defaultMode: 0755
      containers:
        - name: argocd-application-controller
          volumeMounts:
            - name: auth-script
              mountPath: /usr/local/bin/custom-auth
'
```

## 7. Create ApplicationSet

Apply ApplicationSet:
```yaml
kubectl apply -n argocd -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: guestbook
  namespace: argocd
spec:
  generators:
  - clusters: {}
  goTemplate: true
  template:
    metadata:
      name: 'guestbook-{{ .nameNormalized }}'
    spec:
      project: "default"
      source:
        repoURL: https://github.com/argoproj/argocd-example-apps.git
        targetRevision: HEAD
        path: guestbook
      destination:
        server: '{{ .server }}'
        namespace: guestbook
EOF
```

Everything should now be in place!
Verify that the application was created and synced:
```bash
kubectl config use-context kind-spoke
kubectl get pods -n guestbook
```
## 8. Cleanup

```bash
kind delete cluster --name hub
kind delete cluster --name spoke
```

For a similar example, see the ClusterProfile API's [secretreader](https://github.com/kubernetes-sigs/cluster-inventory-api/blob/main/examples/controller-example/README.md).

<!-- TODO: add simple cloud version (very simple execConfig with gcp) -->
<!-- TODO: integrate with `argocd add`, which creates a secret by default -->
<!-- TODO: providers file as secret or configmap? -->