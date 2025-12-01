# Using Cluster Profiles with GCP

This guide demonstrates how to use Cluster Profiles to connect a GKE spoke cluster to an Argo CD instance running in a GKE hub cluster.

## Prerequisites

`gcloud` CLI, GCP project with billing enabled, Kubectl

## 1. Set up environment variables

Set environment variables for your GCP project and desired region. Replace `"your-gcp-project-id"` with your GCP project ID.

```bash
export GCP_PROJECT_ID="your-gcp-project-id"
export GCP_LOCATION="us-central1"
gcloud config set project ${GCP_PROJECT_ID}
gcloud config set compute/region ${GCP_LOCATION}
```

## 2. Create Hub and Spoke GKE Clusters

Create a `hub` cluster with relevant settings:
```bash
gcloud container clusters create hub \
  --location=${GCP_LOCATION} \
  --workload-pool=${GCP_PROJECT_ID}.svc.id.goog \
  --enable-fleet \
  --labels=fleet-clusterinventory-management-cluster=true,fleet-clusterinventory-namespace=argocd,fleet-clusterinventory-access-provider-name=gcp
```
Workload Identity allows Kubernetes service accounts to impersonate GCP service accounts.
Enabling Fleet with the cluster labels has the [Fleet Cluster Profile Syncer](https://docs.cloud.google.com/kubernetes-engine/fleet-management/docs/generate-inventory-for-integrations) create Cluster Profiles automatically for all clusters in the Fleet within the management cluster (`fleet-clusterinventory-management-cluster=true`).

Create a standard GKE Fleet cluster to act as the `spoke`:
```bash
gcloud container clusters create spoke --zone ${GCP_LOCATION} --enable-fleet \
  --labels=fleet-clusterinventory-access-provider-name=gcp
```

Get contexts for both clusters (note `namespace=argocd` for all future `hub` cluster commands):
```bash
gcloud container clusters get-credentials hub --zone ${GCP_LOCATION}
kubectl config set-context --current --namespace=argocd
gcloud container clusters get-credentials spoke --zone ${GCP_LOCATION}
```

Create the namespace for the sample application:
```bash
kubectl config use-context gke_${GCP_PROJECT_ID}_${GCP_LOCATION}_spoke
kubectl create namespace guestbook
```

## 3. Install Argo CD on Hub

Switch to the hub cluster's context and install Argo CD:
```bash
kubectl config use-context gke_${GCP_PROJECT_ID}_${GCP_LOCATION}_hub
kubectl create namespace argocd
kubectl apply -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

If doing local development in a fork of the argo-cd repo, instead build a local image and push it to GCP Artifact Registry:
```bash
# Create an artifact registry repo
gcloud services enable artifactregistry.googleapis.com
export REPO_NAME="argocd-repo"
gcloud artifacts repositories create ${REPO_NAME} --repository-format=docker --location=${GCP_LOCATION}
# Build and push the image
kubectl config use-context gke_${GCP_PROJECT_ID}_${GCP_LOCATION}_hub
kubectl create namespace argocd
gcloud auth configure-docker ${GCP_LOCATION}-docker.pkg.dev
export IMAGE_NAMESPACE=${GCP_LOCATION}-docker.pkg.dev/${GCP_PROJECT_ID}/${REPO_NAME}
export IMAGE_TAG=my-dev-v1
export DOCKER_PUSH=true
make image
make manifests-local
kubectl apply -f manifests/install.yaml
```

## 4. Configure Service Accounts and Permissions


### Hub Cluster Workload Identity

Create a GCP Service Account (GSA) that the ApplicationSet controller will impersonate:
```bash
GSA_NAME="argocd-application-gsa"
gcloud iam service-accounts create ${GSA_NAME} --project=${GCP_PROJECT_ID} --display-name="Argo CD Controller GSA"
```

Grant GSA permissions, if prompted select `condition=None`:
```bash
GSA_EMAIL=${GSA_NAME}@${GCP_PROJECT_ID}.iam.gserviceaccount.com
gcloud projects add-iam-policy-binding ${GCP_PROJECT_ID} --member="serviceAccount:${GSA_EMAIL}" --role="roles/container.developer"
gcloud projects add-iam-policy-binding ${GCP_PROJECT_ID} --member="serviceAccount:${GSA_EMAIL}" --role="roles/gkehub.gatewayAdmin"
```

Allow Argo Application service account to impersonate GSA
```bash
gcloud iam service-accounts add-iam-policy-binding ${GSA_EMAIL} --project=${GCP_PROJECT_ID} --role="roles/iam.workloadIdentityUser" --member="serviceAccount:${GCP_PROJECT_ID}.svc.id.goog[argocd/argocd-application-controller]"
```

Annotate the Application controller KSA with its impersonated GSA:
```bash
kubectl annotate serviceaccount argocd-application-controller "iam.gke.io/gcp-service-account=${GSA_EMAIL}" --overwrite
```

## 5. Create Access Providers Secret in Hub

Create a secret in the `hub` cluster containing an `execConfig`. This config tells Argo CD to run `argocd-k8s-auth gcp` to obtain credentials for clusters. When this command runs from the ApplicationSet controller pod, it will automatically use the configured Workload Identity.

```bash
kubectl config use-context gke_${GCP_PROJECT_ID}_${GCP_LOCATION}_hub
kubectl create secret generic cp-creds-secret \
  --from-file=cp-creds.json=/dev/stdin <<EOF
{
  "providers": [
    {
      "name": "gcp",
      "execConfig": {
        "command": "argocd-k8s-auth",
        "args": ["gcp"],
        "apiVersion": "client.authentication.k8s.io/v1beta1"
      }
    }
  ]
}
EOF
```

Mount the `cp-creds-secret` into the `argocd-applicationset-controller` and pass the file path as a command-line argument. This enables the ApplicationSet controller's Cluster Profile syncer.
```bash
kubectl patch deploy/argocd-applicationset-controller --type strategic --patch '
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
            - "--cluster-profile-providers-file=/app/cp-creds/cp-creds.json"'
```

Restart the controller to ensure it uses the Workload Identity annotation and mounts the new secret.
```bash
kubectl rollout restart deployment argocd-applicationset-controller
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=argocd-applicationset-controller --timeout=300s
```

## 9. Create ApplicationSet

At this point, the Cluster Profile and Secret should be generated.

With the cluster connection configured, create an `ApplicationSet`:
```yaml
kubectl apply -f - <<EOF
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

## 10. Verify

Verify that the `guestbook` application was deployed to the `spoke` cluster.
```bash
kubectl config use-context gke_${GCP_PROJECT_ID}_${GCP_LOCATION}_spoke
kubectl get pods -n guestbook
```
You should see the `guestbook-ui` pod running.

## 11. Cleanup

Delete the GKE clusters.
```bash
gcloud container clusters delete hub --zone ${GCP_LOCATION} --quiet --async
gcloud container clusters delete spoke --zone ${GCP_LOCATION} --quiet --async
```

Delete the GCP Service Account.
```bash
gcloud iam service-accounts delete ${GSA_EMAIL} --quiet
```

Delete the Artifact Registry repo.
```bash
gcloud artifacts repositories delete ${REPO_NAME} --location=${GCP_LOCATION} --quiet
```