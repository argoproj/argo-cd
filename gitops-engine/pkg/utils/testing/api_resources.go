package testing

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

var (
	CommonVerbs      = metav1.Verbs{"create", "get", "list", "watch", "update", "patch", "delete", "deletecollection"}
	subresourceVerbs = metav1.Verbs{"get", "update", "patch"}
)

// StaticAPIResources defines the common Kubernetes API resources that are usually returned by a DiscoveryClient
var StaticAPIResources = []*metav1.APIResourceList{
	{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{
			{Name: "pods", SingularName: "pod", Namespaced: true, Kind: "Pod", Verbs: CommonVerbs},
			{Name: "pods/status", SingularName: "", Namespaced: true, Kind: "Pod", Verbs: subresourceVerbs},
			{Name: "pods/log", SingularName: "", Namespaced: true, Kind: "Pod", Verbs: metav1.Verbs{"get"}},
			{Name: "pods/exec", SingularName: "", Namespaced: true, Kind: "Pod", Verbs: metav1.Verbs{"create"}},
			{Name: "services", SingularName: "service", Namespaced: true, Kind: "Service", Verbs: CommonVerbs},
			{Name: "services/status", SingularName: "", Namespaced: true, Kind: "Service", Verbs: subresourceVerbs},
			{Name: "configmaps", SingularName: "configmap", Namespaced: true, Kind: "ConfigMap", Verbs: CommonVerbs},
			{Name: "secrets", SingularName: "secret", Namespaced: true, Kind: "Secret", Verbs: CommonVerbs},
			{Name: "namespaces", SingularName: "namespace", Namespaced: false, Kind: "Namespace", Verbs: CommonVerbs},
			{Name: "namespaces/status", SingularName: "", Namespaced: false, Kind: "Namespace", Verbs: subresourceVerbs},
			{Name: "nodes", SingularName: "node", Namespaced: false, Kind: "Node", Verbs: metav1.Verbs{"get", "list", "watch"}},
			{Name: "persistentvolumes", SingularName: "persistentvolume", Namespaced: false, Kind: "PersistentVolume", Verbs: CommonVerbs},
			{Name: "persistentvolumeclaims", SingularName: "persistentvolumeclaim", Namespaced: true, Kind: "PersistentVolumeClaim", Verbs: CommonVerbs},
			{Name: "persistentvolumeclaims/status", SingularName: "", Namespaced: true, Kind: "PersistentVolumeClaim", Verbs: subresourceVerbs},
			{Name: "events", SingularName: "event", Namespaced: true, Kind: "Event", Verbs: metav1.Verbs{"create", "get", "list", "watch"}},
			{Name: "serviceaccounts", SingularName: "serviceaccount", Namespaced: true, Kind: "ServiceAccount", Verbs: CommonVerbs},
		},
	},
	{
		GroupVersion: "apps/v1",
		APIResources: []metav1.APIResource{
			{Name: "deployments", SingularName: "deployment", Namespaced: true, Kind: "Deployment", Verbs: CommonVerbs},
			{Name: "deployments/status", SingularName: "", Namespaced: true, Kind: "Deployment", Verbs: subresourceVerbs},
			{Name: "deployments/scale", SingularName: "", Namespaced: true, Kind: "Scale", Verbs: subresourceVerbs},
			{Name: "statefulsets", SingularName: "statefulset", Namespaced: true, Kind: "StatefulSet", Verbs: CommonVerbs},
			{Name: "statefulsets/status", SingularName: "", Namespaced: true, Kind: "StatefulSet", Verbs: subresourceVerbs},
			{Name: "statefulsets/scale", SingularName: "", Namespaced: true, Kind: "Scale", Verbs: subresourceVerbs},
			{Name: "daemonsets", SingularName: "daemonset", Namespaced: true, Kind: "DaemonSet", Verbs: CommonVerbs},
			{Name: "daemonsets/status", SingularName: "", Namespaced: true, Kind: "DaemonSet", Verbs: subresourceVerbs},
			{Name: "replicasets", SingularName: "replicaset", Namespaced: true, Kind: "ReplicaSet", Verbs: CommonVerbs},
			{Name: "replicasets/status", SingularName: "", Namespaced: true, Kind: "ReplicaSet", Verbs: subresourceVerbs},
		},
	},
	{
		GroupVersion: "batch/v1",
		APIResources: []metav1.APIResource{
			{Name: "jobs", SingularName: "job", Namespaced: true, Kind: "Job", Verbs: CommonVerbs},
			{Name: "jobs/status", SingularName: "", Namespaced: true, Kind: "Job", Verbs: subresourceVerbs},
			{Name: "cronjobs", SingularName: "cronjob", Namespaced: true, Kind: "CronJob", Verbs: CommonVerbs},
			{Name: "cronjobs/status", SingularName: "", Namespaced: true, Kind: "CronJob", Verbs: subresourceVerbs},
		},
	},
	{
		GroupVersion: "rbac.authorization.k8s.io/v1",
		APIResources: []metav1.APIResource{
			{Name: "roles", SingularName: "role", Namespaced: true, Kind: "Role", Verbs: CommonVerbs},
			{Name: "rolebindings", SingularName: "rolebinding", Namespaced: true, Kind: "RoleBinding", Verbs: CommonVerbs},
			{Name: "clusterroles", SingularName: "clusterrole", Namespaced: false, Kind: "ClusterRole", Verbs: CommonVerbs},
			{Name: "clusterrolebindings", SingularName: "clusterrolebinding", Namespaced: false, Kind: "ClusterRoleBinding", Verbs: CommonVerbs},
		},
	},
	{
		GroupVersion: "networking.k8s.io/v1",
		APIResources: []metav1.APIResource{
			{Name: "ingresses", SingularName: "ingress", Namespaced: true, Kind: "Ingress", Verbs: CommonVerbs},
			{Name: "ingresses/status", SingularName: "", Namespaced: true, Kind: "Ingress", Verbs: subresourceVerbs},
			{Name: "networkpolicies", SingularName: "networkpolicy", Namespaced: true, Kind: "NetworkPolicy", Verbs: CommonVerbs},
		},
	},
	{
		GroupVersion: "policy/v1",
		APIResources: []metav1.APIResource{
			{Name: "poddisruptionbudgets", SingularName: "poddisruptionbudget", Namespaced: true, Kind: "PodDisruptionBudget", Verbs: CommonVerbs},
			{Name: "poddisruptionbudgets/status", SingularName: "", Namespaced: true, Kind: "PodDisruptionBudget", Verbs: subresourceVerbs},
		},
	},
	{
		GroupVersion: "apiextensions.k8s.io/v1",
		APIResources: []metav1.APIResource{
			{Kind: "CustomResourceDefinition", Group: "apiextensions.k8s.io", Version: "v1", Namespaced: false, Verbs: CommonVerbs},
		},
	},
}
