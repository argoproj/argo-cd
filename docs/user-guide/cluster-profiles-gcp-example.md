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
Enabling Fleet with the cluster labels tells the [Fleet Cluster Profile Syncer](https://docs.cloud.google.com/kubernetes-engine/fleet-management/docs/generate-inventory-for-integrations) to automatically create Cluster Profiles for all clusters in the Fleet within the management cluster (`fleet-clusterinventory-management-cluster=true`). The `fleet-clusterinventory-access-provider-name=argo-cd-builtin-gcp` label tells the ClusterProfile to use the access provider name `argo-cd-builtin-gcp`, this `argo-cd-builtin-` prefix indicates that the Cluster Profile controller should generate a secret configured for built-in GCP authentication rather than look for a custom access providers file. For an example with a custom exec config, see the [kind example](cluster-profiles-kind-example.md).

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
kubectl create namespace guestbook
```

## 3. Install Argo CD on Hub

Install Argo CD in the hub cluster:
```bash
kubectl config use-context gke_${GCP_PROJECT_ID}_${GCP_LOCATION}_hub
kubectl create namespace argocd
# Install Argo CD. ApplicationSet CRD is too large for client-side `kubectl apply`, use server-side:
kubectl apply --server-side --force-conflicts -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# Enable the Cluster Profile Controller
kubectl scale deployment argocd-clusterprofile-controller --replicas=1 -n argocd
```

### \[Alternative\] Local Development

To include local changes to the Argo CD source code in a local fork of the argo-cd repository, instead build a local image and push it to GCP Artifact Registry:

```bash
kubectl config use-context gke_${GCP_PROJECT_ID}_${GCP_LOCATION}_hub
kubectl create namespace argocd
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
# Install local Argo manifests. ApplicationSet CRD is too large for client-side `kubectl apply`, use server-side:
kubectl apply --server-side --force-conflicts -f manifests/install.yaml

# Enable the Cluster Profile Controller
kubectl scale deployment argocd-clusterprofile-controller --replicas=1
```

To update with new changes:

```bash
make image
make manifests-local
kubectl apply --server-side --force-conflicts -f manifests/install.yaml
kubectl scale deployment argocd-clusterprofile-controller --replicas=1
```

## 4. Configure Service Accounts and Permissions


### Hub Cluster Workload Identity

For both the Cluster Profile and Application controller, create a GCP Service Account (GSA) that the controllers will impersonate:
```bash
export GSA_NAME_APP="argocd-application-gsa"
export GSA_NAME_PROFILE="argocd-clusterprofile-gsa"
gcloud iam service-accounts create ${GSA_NAME_APP} --project=${GCP_PROJECT_ID} --display-name="Argo CD Application Controller GSA"
gcloud iam service-accounts create ${GSA_NAME_PROFILE} --project=${GCP_PROJECT_ID} --display-name="Argo CD ClusterProfile Controller GSA"
```

Grant permissions to the GSAs:
```bash
export GSA_EMAIL_APP=${GSA_NAME_APP}@${GCP_PROJECT_ID}.iam.gserviceaccount.com
export GSA_EMAIL_PROFILE=${GSA_NAME_PROFILE}@${GCP_PROJECT_ID}.iam.gserviceaccount.com
gcloud projects add-iam-policy-binding ${GCP_PROJECT_ID} --member="serviceAccount:${GSA_EMAIL_APP}" --role="roles/container.developer" --condition=None
gcloud projects add-iam-policy-binding ${GCP_PROJECT_ID} --member="serviceAccount:${GSA_EMAIL_APP}" --role="roles/container.clusterAdmin" --condition=None
gcloud projects add-iam-policy-binding ${GCP_PROJECT_ID} --member="serviceAccount:${GSA_EMAIL_APP}" --role="roles/gkehub.gatewayAdmin" --condition=None
gcloud projects add-iam-policy-binding ${GCP_PROJECT_ID} --member="serviceAccount:${GSA_EMAIL_PROFILE}" --role="roles/container.developer" --condition=None
gcloud projects add-iam-policy-binding ${GCP_PROJECT_ID} --member="serviceAccount:${GSA_EMAIL_PROFILE}" --role="roles/container.clusterAdmin" --condition=None
gcloud projects add-iam-policy-binding ${GCP_PROJECT_ID} --member="serviceAccount:${GSA_EMAIL_PROFILE}" --role="roles/gkehub.gatewayAdmin" --condition=None
```

Give the controller service accounts permissions and annotations to impersonate the GSAs:
```bash
gcloud iam service-accounts add-iam-policy-binding ${GSA_EMAIL_APP} --project=${GCP_PROJECT_ID} --role="roles/iam.workloadIdentityUser" --member="serviceAccount:${GCP_PROJECT_ID}.svc.id.goog[argocd/argocd-application-controller]"
kubectl annotate serviceaccount argocd-application-controller "iam.gke.io/gcp-service-account=${GSA_EMAIL_APP}" --overwrite
gcloud iam service-accounts add-iam-policy-binding ${GSA_EMAIL_PROFILE} --project=${GCP_PROJECT_ID} --role="roles/iam.workloadIdentityUser" --member="serviceAccount:${GCP_PROJECT_ID}.svc.id.goog[argocd/argocd-clusterprofile-controller]"
kubectl annotate serviceaccount argocd-clusterprofile-controller "iam.gke.io/gcp-service-account=${GSA_EMAIL_PROFILE}" --overwrite

kubectl rollout restart statefulset argocd-application-controller
kubectl rollout restart deployment argocd-clusterprofile-controller
```

### Spoke Cluster RBAC

The Application Controller will authenticate to the spoke cluster using Connect Gateway, presenting its Workload Identity to the spoke cluster. Grant it `cluster-admin` access on the spoke cluster:

```bash
kubectl config use-context gke_${GCP_PROJECT_ID}_${GCP_LOCATION}_spoke
kubectl create clusterrolebinding argocd-application-controller-admin \
  --clusterrole=cluster-admin \
  --user="serviceAccount:${GCP_PROJECT_ID}.svc.id.goog[argocd/argocd-application-controller]"
kubectl config use-context gke_${GCP_PROJECT_ID}_${GCP_LOCATION}_hub
```

## 5. Create ApplicationSet

At this point, the Cluster Profile and Secret should be generated (you may verify with `kubectl get clusterprofiles` and `kubectl get secrets`). Argo CD will use the built-in GCP provider to authenticate to the spoke cluster using Workload Identity.

With the cluster connection configured, create an `ApplicationSet`:
```bash
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
echo -e "\nClusterProfile controller errors:" && kubectl logs deployment/argocd-clusterprofile-controller | grep Error
echo -e "\nApplication controller errors:" && kubectl logs statefulset/argocd-application-controller | grep Error
echo -e "\nController:" && kubectl get pods | grep clusterprofile-controller
echo -e "\nClusterProfile:" && kubectl get clusterprofiles | grep spoke-us-central1
echo -e "\nSecret:" && kubectl get secrets | grep cluster-spoke-us-central1
echo -e "\nApplication:" && kubectl get applications | grep guestbook-spoke-us-central1
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

Delete the GCP Service Accounts.
```bash
gcloud iam service-accounts delete ${GSA_EMAIL_APP} --quiet
gcloud iam service-accounts delete ${GSA_EMAIL_PROFILE} --quiet
```

Delete the Artifact Registry repo.
```bash
gcloud artifacts repositories delete ${REPO_NAME} --location=${GCP_LOCATION} --quiet --async
```