package testing

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

var (
	commonVerbs      = []string{"create", "get", "list", "watch", "update", "patch", "delete", "deletecollection"}
	subresourceVerbs = []string{"get", "update", "patch"}
)

// StaticAPIResources defines the common Kubernetes API resources that are usually returned by a DiscoveryClient
var StaticAPIResources = []*metav1.APIResourceList{
	{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{
			{Name: "pods", SingularName: "pod", Namespaced: true, Kind: "Pod", Verbs: commonVerbs},
			{Name: "pods/status", SingularName: "", Namespaced: true, Kind: "Pod", Verbs: subresourceVerbs},
			{Name: "pods/log", SingularName: "", Namespaced: true, Kind: "Pod", Verbs: []string{"get"}},
			{Name: "pods/exec", SingularName: "", Namespaced: true, Kind: "Pod", Verbs: []string{"create"}},
			{Name: "services", SingularName: "service", Namespaced: true, Kind: "Service", Verbs: commonVerbs},
			{Name: "services/status", SingularName: "", Namespaced: true, Kind: "Service", Verbs: subresourceVerbs},
			{Name: "configmaps", SingularName: "configmap", Namespaced: true, Kind: "ConfigMap", Verbs: commonVerbs},
			{Name: "secrets", SingularName: "secret", Namespaced: true, Kind: "Secret", Verbs: commonVerbs},
			{Name: "namespaces", SingularName: "namespace", Namespaced: false, Kind: "Namespace", Verbs: commonVerbs},
			{Name: "namespaces/status", SingularName: "", Namespaced: false, Kind: "Namespace", Verbs: subresourceVerbs},
			{Name: "nodes", SingularName: "node", Namespaced: false, Kind: "Node", Verbs: []string{"get", "list", "watch"}},
			{Name: "persistentvolumes", SingularName: "persistentvolume", Namespaced: false, Kind: "PersistentVolume", Verbs: commonVerbs},
			{Name: "persistentvolumeclaims", SingularName: "persistentvolumeclaim", Namespaced: true, Kind: "PersistentVolumeClaim", Verbs: commonVerbs},
			{Name: "persistentvolumeclaims/status", SingularName: "", Namespaced: true, Kind: "PersistentVolumeClaim", Verbs: subresourceVerbs},
			{Name: "events", SingularName: "event", Namespaced: true, Kind: "Event", Verbs: []string{"create", "get", "list", "watch"}},
			{Name: "serviceaccounts", SingularName: "serviceaccount", Namespaced: true, Kind: "ServiceAccount", Verbs: commonVerbs},
		},
	},
	{
		GroupVersion: "apps/v1",
		APIResources: []metav1.APIResource{
			{Name: "deployments", SingularName: "deployment", Namespaced: true, Kind: "Deployment", Verbs: commonVerbs},
			{Name: "deployments/status", SingularName: "", Namespaced: true, Kind: "Deployment", Verbs: subresourceVerbs},
			{Name: "deployments/scale", SingularName: "", Namespaced: true, Kind: "Scale", Verbs: subresourceVerbs},
			{Name: "statefulsets", SingularName: "statefulset", Namespaced: true, Kind: "StatefulSet", Verbs: commonVerbs},
			{Name: "statefulsets/status", SingularName: "", Namespaced: true, Kind: "StatefulSet", Verbs: subresourceVerbs},
			{Name: "statefulsets/scale", SingularName: "", Namespaced: true, Kind: "Scale", Verbs: subresourceVerbs},
			{Name: "daemonsets", SingularName: "daemonset", Namespaced: true, Kind: "DaemonSet", Verbs: commonVerbs},
			{Name: "daemonsets/status", SingularName: "", Namespaced: true, Kind: "DaemonSet", Verbs: subresourceVerbs},
			{Name: "replicasets", SingularName: "replicaset", Namespaced: true, Kind: "ReplicaSet", Verbs: commonVerbs},
			{Name: "replicasets/status", SingularName: "", Namespaced: true, Kind: "ReplicaSet", Verbs: subresourceVerbs},
		},
	},
	{
		GroupVersion: "batch/v1",
		APIResources: []metav1.APIResource{
			{Name: "jobs", SingularName: "job", Namespaced: true, Kind: "Job", Verbs: commonVerbs},
			{Name: "jobs/status", SingularName: "", Namespaced: true, Kind: "Job", Verbs: subresourceVerbs},
			{Name: "cronjobs", SingularName: "cronjob", Namespaced: true, Kind: "CronJob", Verbs: commonVerbs},
			{Name: "cronjobs/status", SingularName: "", Namespaced: true, Kind: "CronJob", Verbs: subresourceVerbs},
		},
	},
	{
		GroupVersion: "rbac.authorization.k8s.io/v1",
		APIResources: []metav1.APIResource{
			{Name: "roles", SingularName: "role", Namespaced: true, Kind: "Role", Verbs: commonVerbs},
			{Name: "rolebindings", SingularName: "rolebinding", Namespaced: true, Kind: "RoleBinding", Verbs: commonVerbs},
			{Name: "clusterroles", SingularName: "clusterrole", Namespaced: false, Kind: "ClusterRole", Verbs: commonVerbs},
			{Name: "clusterrolebindings", SingularName: "clusterrolebinding", Namespaced: false, Kind: "ClusterRoleBinding", Verbs: commonVerbs},
		},
	},
	{
		GroupVersion: "networking.k8s.io/v1",
		APIResources: []metav1.APIResource{
			{Name: "ingresses", SingularName: "ingress", Namespaced: true, Kind: "Ingress", Verbs: commonVerbs},
			{Name: "ingresses/status", SingularName: "", Namespaced: true, Kind: "Ingress", Verbs: subresourceVerbs},
			{Name: "networkpolicies", SingularName: "networkpolicy", Namespaced: true, Kind: "NetworkPolicy", Verbs: commonVerbs},
		},
	},
	{
		GroupVersion: "policy/v1",
		APIResources: []metav1.APIResource{
			{Name: "poddisruptionbudgets", SingularName: "poddisruptionbudget", Namespaced: true, Kind: "PodDisruptionBudget", Verbs: commonVerbs},
			{Name: "poddisruptionbudgets/status", SingularName: "", Namespaced: true, Kind: "PodDisruptionBudget", Verbs: subresourceVerbs},
		},
	},
}
