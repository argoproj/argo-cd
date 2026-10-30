This folder contains example RBAC for Kubernetes to allow the Argo CD API
Server (`argocd-server`) and the Argo CD Application Controller
(`argocd-application-controller`) to perform operations on `Application`
CRs in all namespaces on the cluster.

Applying the `ClusterRole` and `ClusterRoleBinding` resources in this
folder grant the Argo CD API server and the Argo CD Application
Controller read and write permissions on `Application` resources
cluster-wide, which may not be what you want. Handle with care.

Only apply these if you have installed Argo CD into the default namespace
`argocd`. Otherwise, you need to edit the cluster role binding to bind to
the service account in the correct namespace.