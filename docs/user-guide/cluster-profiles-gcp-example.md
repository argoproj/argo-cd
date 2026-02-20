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
  --labels=fleet-clusterinventory-management-cluster=true,fleet-clusterinventory-namespace=argocd,fleet-clusterinventory-access-provider-name=argo-cd-builtin-gcp
```

Workload Identity allows Kubernetes service accounts to impersonate GCP service accounts.
Enabling Fleet with the cluster labels tells the [Fleet Cluster Profile Syncer](https://docs.cloud.google.com/kubernetes-engine/fleet-management/docs/generate-inventory-for-integrations) to automatically create Cluster Profiles for all clusters in the Fleet within the management cluster (`fleet-clusterinventory-management-cluster=true`). The `fleet-clusterinventory-access-provider-name=argo-cd-builtin-gcp` label tells the ClusterProfile to use the access provider name indicating that the ApplicationSet controller should use built-in GCP authentication.

Create a standard GKE Fleet cluster to act as the `spoke`:
```bash
gcloud container clusters create spoke --location=${GCP_LOCATION} --enable-fleet \
  --labels=fleet-clusterinventory-access-provider-name=argo-cd-builtin-gcp
```

Get contexts for both clusters and set `namespace=argocd` for all future `hub` cluster commands:
```bash
gcloud container clusters get-credentials hub --location=${GCP_LOCATION}
kubectl config set-context --current --namespace=argocd
gcloud container clusters get-credentials spoke --location=${GCP_LOCATION}
```

Create the namespace for the sample application in the spoke cluster:
```bash
kubectl config use-context gke_${GCP_PROJECT_ID}_${GCP_LOCATION}_spoke
kubectl create namespace guestbook
```

## 3. Install Argo CD on Hub

Install Argo CD in the hub cluster:
```bash
kubectl config use-context gke_${GCP_PROJECT_ID}_${GCP_LOCATION}_hub
kubectl create namespace argocd
kubectl create -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
kubectl set env deployment/argocd-applicationset-controller ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_CLUSTER_PROFILES="true"
```

### \[Alternative\] Local Development

For testing local changes in a fork of the argo-cd repository, instead build a local image and push it to GCP Artifact Registry:

```bash
# Create an artifact registry repo
gcloud services enable artifactregistry.googleapis.com
export REPO_NAME="argocd-repo"
gcloud artifacts repositories create ${REPO_NAME} --repository-format=docker --location=${GCP_LOCATION}
# Build and push the image
kubectl config use-context gke_${GCP_PROJECT_ID}_${GCP_LOCATION}_hub
gcloud auth configure-docker ${GCP_LOCATION}-docker.pkg.dev
export IMAGE_NAMESPACE=${GCP_LOCATION}-docker.pkg.dev/${GCP_PROJECT_ID}/${REPO_NAME}
export IMAGE_TAG=my-dev-v1
export DOCKER_PUSH=true
make image
make manifests-local
# ApplicationSet definition is too large for `kubectl apply`.
kubectl create -f manifests/install.yaml
kubectl set env deployment/argocd-applicationset-controller ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_CLUSTER_PROFILES="true"
```

To update with new changes:

```bash
make image
make manifests-local
kubectl replace -f manifests/install.yaml
kubectl set env deployment/argocd-applicationset-controller ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_CLUSTER_PROFILES="true"
kubectl rollout restart deployment argocd-applicationset-controller
```

## 4. Configure Service Accounts and Permissions


### Hub Cluster Workload Identity

Create a GCP Service Account (GSA) that the ApplicationSet controller will impersonate:
```bash
GSA_NAME="argocd-application-gsa"
gcloud iam service-accounts create ${GSA_NAME} --project=${GCP_PROJECT_ID} --display-name="Argo CD Controller GSA"
```

Grant permissions to the GSA:
```bash
GSA_EMAIL=${GSA_NAME}@${GCP_PROJECT_ID}.iam.gserviceaccount.com
gcloud projects add-iam-policy-binding ${GCP_PROJECT_ID} --member="serviceAccount:${GSA_EMAIL}" --role="roles/container.developer" --condition=None
gcloud projects add-iam-policy-binding ${GCP_PROJECT_ID} --member="serviceAccount:${GSA_EMAIL}" --role="roles/container.clusterAdmin" --condition=None
gcloud projects add-iam-policy-binding ${GCP_PROJECT_ID} --member="serviceAccount:${GSA_EMAIL}" --role="roles/gkehub.gatewayAdmin" --condition=None
```

Allow the Argo Application Controller service account to impersonate the GSA and annotate this KSA with its GSA:
```bash
gcloud iam service-accounts add-iam-policy-binding ${GSA_EMAIL} --project=${GCP_PROJECT_ID} --role="roles/iam.workloadIdentityUser" --member="serviceAccount:${GCP_PROJECT_ID}.svc.id.goog[argocd/argocd-application-controller]"
kubectl annotate serviceaccount argocd-application-controller "iam.gke.io/gcp-service-account=${GSA_EMAIL}" --overwrite
```

Restart the controllers:
```bash
kubectl rollout restart deployment argocd-applicationset-controller
kubectl rollout restart statefulset argocd-application-controller
```

## 5. Create ApplicationSet

At this point, the Cluster Profile and Secret should be generated (you may verify with `kubectl get clusterprofiles` and `kubectl get secrets`). The ApplicationSet controller will use the built-in GCP provider to authenticate to the spoke cluster using Workload Identity.

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

## 6. Sync

Trigger the application to sync:
```bash
kubectl patch application guestbook-spoke-${GCP_LOCATION} -p '{"operation": {"sync": {"prune": true}}}' --type=merge
```

Verify that the `guestbook` application was deployed to the `spoke` cluster:
```bash
kubectl config use-context gke_${GCP_PROJECT_ID}_${GCP_LOCATION}_spoke
kubectl get pods -n guestbook
```

If you should see a `guestbook-ui` pod running, congratulations on completing this guide! You now have an automatic flow which prepares any new cluster in the Fleet to be synced automatically through a Cluster Profile, secret, and application.

If not, debug:
```bash
kubectl config use-context gke_${GCP_PROJECT_ID}_${GCP_LOCATION}_hub
kubectl get secrets
kubectl get applications
kubectl logs argocd-application-controller-0
```

* If you see permission issues when connecting to the cluster, check that you didn't miss any of Step 4.
* If you see server issues (`server.secretkey is missing`), restart the server (`kubectl rollout restart deploy/argocd-server`).
* If everything looks correct, try triggering the sync in the Argo CD UI.

## 7. Cleanup

Delete the GKE clusters.
```bash
gcloud container clusters delete hub --location=${GCP_LOCATION} --quiet --async
gcloud container clusters delete spoke --location=${GCP_LOCATION} --quiet --async
```

Delete the GCP Service Account.
```bash
gcloud iam service-accounts delete ${GSA_EMAIL} --quiet
```

Delete the Artifact Registry repo.
```bash
gcloud artifacts repositories delete ${REPO_NAME} --location=${GCP_LOCATION} --quiet
```