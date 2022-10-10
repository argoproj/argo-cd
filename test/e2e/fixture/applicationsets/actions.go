package applicationsets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/argoproj/argo-cd/v2/common"
	argocommon "github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture/applicationsets/utils"
	"github.com/argoproj/argo-cd/v2/util/clusterauth"
)

// this implements the "when" part of given/when/then
//
// none of the func implement error checks, and that is complete intended, you should check for errors
// using the Then()
type Actions struct {
	context        *Context
	lastOutput     string
	lastError      error
	describeAction string
	ignoreErrors   bool
}

var pdGVR = schema.GroupVersionResource{
	Group:    "cluster.open-cluster-management.io",
	Version:  "v1alpha1",
	Resource: "placementdecisions",
}

// IgnoreErrors sets whether to ignore
func (a *Actions) IgnoreErrors() *Actions {
	a.ignoreErrors = true
	return a
}

func (a *Actions) DoNotIgnoreErrors() *Actions {
	a.ignoreErrors = false
	return a
}

func (a *Actions) And(block func()) *Actions {
	a.context.t.Helper()
	block()
	return a
}

func (a *Actions) Then() *Consequences {
	a.context.t.Helper()
	return &Consequences{a.context, a}
}

// CreateClusterSecret creates a faux cluster secret, with the given cluster server and cluster name (this cluster
// will not actually be used by the Argo CD controller, but that's not needed for our E2E tests)
func (a *Actions) CreateClusterSecret(secretName string, clusterName string, clusterServer string) *Actions {

	fixtureClient := utils.GetE2EFixtureK8sClient()

	var serviceAccountName string

	// Look for a service account matching '*application-controller*'
	err := wait.Poll(500*time.Millisecond, 30*time.Second, func() (bool, error) {

		serviceAccountList, err := fixtureClient.KubeClientset.CoreV1().ServiceAccounts(utils.ArgoCDNamespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			fmt.Println("Unable to retrieve ServiceAccount list", err)
			return false, nil
		}

		// If 'application-controller' service account is present, use that
		for _, sa := range serviceAccountList.Items {
			if strings.Contains(sa.Name, "application-controller") {
				serviceAccountName = sa.Name
				return true, nil
			}
		}

		// Otherwise, use 'default'
		for _, sa := range serviceAccountList.Items {
			if sa.Name == "default" {
				serviceAccountName = sa.Name
				return true, nil
			}
		}

		return false, nil
	})

	if err == nil {
		var bearerToken string
		bearerToken, err = clusterauth.GetServiceAccountBearerToken(fixtureClient.KubeClientset, utils.ArgoCDNamespace, serviceAccountName, common.BearerTokenTimeout)

		// bearerToken
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: utils.ArgoCDNamespace,
				Labels: map[string]string{
					argocommon.LabelKeySecretType: argocommon.LabelValueSecretTypeCluster,
					utils.TestingLabel:            "true",
				},
			},
			Data: map[string][]byte{
				"name":   []byte(clusterName),
				"server": []byte(clusterServer),
				"config": []byte("{\"username\":\"foo\",\"password\":\"foo\"}"),
			},
		}

		// If the bearer token is available, use it rather than the fake username/password
		if bearerToken != "" && err == nil {
			secret.Data = map[string][]byte{
				"name":   []byte(clusterName),
				"server": []byte(clusterServer),
				"config": []byte("{\"bearerToken\":\"" + bearerToken + "\"}"),
			}
		}

		_, err = fixtureClient.KubeClientset.CoreV1().Secrets(secret.Namespace).Create(context.Background(), secret, metav1.CreateOptions{})
	}

	a.describeAction = fmt.Sprintf("creating cluster Secret '%s'", secretName)
	a.lastOutput, a.lastError = "", err
	a.verifyAction()

	return a
}

// DeleteClusterSecret deletes a faux cluster secret
func (a *Actions) DeleteClusterSecret(secretName string) *Actions {

	err := utils.GetE2EFixtureK8sClient().KubeClientset.CoreV1().Secrets(utils.ArgoCDNamespace).Delete(context.Background(), secretName, metav1.DeleteOptions{})

	a.describeAction = fmt.Sprintf("deleting cluster Secret '%s'", secretName)
	a.lastOutput, a.lastError = "", err
	a.verifyAction()

	return a
}

// DeleteConfigMap deletes a faux cluster secret
func (a *Actions) DeleteConfigMap(configMapName string) *Actions {

	err := utils.GetE2EFixtureK8sClient().KubeClientset.CoreV1().ConfigMaps(utils.ArgoCDNamespace).Delete(context.Background(), configMapName, metav1.DeleteOptions{})

	a.describeAction = fmt.Sprintf("deleting configMap '%s'", configMapName)
	a.lastOutput, a.lastError = "", err
	a.verifyAction()

	return a
}

// DeletePlacementDecision deletes a faux cluster secret
func (a *Actions) DeletePlacementDecision(placementDecisionName string) *Actions {

	err := utils.GetE2EFixtureK8sClient().DynamicClientset.Resource(pdGVR).Namespace(utils.ArgoCDNamespace).Delete(context.Background(), placementDecisionName, metav1.DeleteOptions{})

	a.describeAction = fmt.Sprintf("deleting placement decision '%s'", placementDecisionName)
	a.lastOutput, a.lastError = "", err
	a.verifyAction()

	return a
}

// Create a temporary namespace, from utils.ApplicationSet, for use by the test.
// This namespace will be deleted on subsequent tests.
func (a *Actions) CreateNamespace() *Actions {
	a.context.t.Helper()

	fixtureClient := utils.GetE2EFixtureK8sClient()

	_, err := fixtureClient.KubeClientset.CoreV1().Namespaces().Create(context.Background(),
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: utils.ApplicationSetNamespace}}, metav1.CreateOptions{})

	a.describeAction = fmt.Sprintf("creating namespace '%s'", utils.ApplicationSetNamespace)
	a.lastOutput, a.lastError = "", err
	a.verifyAction()

	return a
}

// Create creates an ApplicationSet using the provided value
func (a *Actions) Create(appSet v1alpha1.ApplicationSet) *Actions {
	a.context.t.Helper()

	appSet.APIVersion = "argoproj.io/v1alpha1"
	appSet.Kind = "ApplicationSet"

	fixtureClient := utils.GetE2EFixtureK8sClient()
	newResource, err := fixtureClient.AppSetClientset.Create(context.Background(), utils.MustToUnstructured(&appSet), metav1.CreateOptions{})

	if err == nil {
		a.context.name = newResource.GetName()
	}

	a.describeAction = fmt.Sprintf("creating ApplicationSet '%s'", appSet.Name)
	a.lastOutput, a.lastError = "", err
	a.verifyAction()

	return a
}

// Create Role/RoleBinding to allow ApplicationSet to list the PlacementDecisions
func (a *Actions) CreatePlacementRoleAndRoleBinding() *Actions {
	fixtureClient := utils.GetE2EFixtureK8sClient()

	var err error

	_, err = fixtureClient.KubeClientset.RbacV1().Roles(utils.ArgoCDNamespace).Create(context.Background(), &v1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "placement-role", Namespace: utils.ArgoCDNamespace},
		Rules: []v1.PolicyRule{
			{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{"cluster.open-cluster-management.io"},
				Resources: []string{"placementdecisions"},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil && strings.Contains(err.Error(), "already exists") {
		err = nil
	}

	if err == nil {
		_, err = fixtureClient.KubeClientset.RbacV1().RoleBindings(utils.ArgoCDNamespace).Create(context.Background(),
			&v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "placement-role-binding", Namespace: utils.ArgoCDNamespace},
				Subjects: []v1.Subject{
					{
						Name:      "argocd-applicationset-controller",
						Namespace: utils.ArgoCDNamespace,
						Kind:      "ServiceAccount",
					},
				},
				RoleRef: v1.RoleRef{
					Kind:     "Role",
					APIGroup: "rbac.authorization.k8s.io",
					Name:     "placement-role",
				},
			}, metav1.CreateOptions{})
	}
	if err != nil && strings.Contains(err.Error(), "already exists") {
		err = nil
	}

	a.describeAction = "creating placement role/rolebinding"
	a.lastOutput, a.lastError = "", err
	a.verifyAction()

	return a
}

// Create a ConfigMap for the ClusterResourceList generator
func (a *Actions) CreatePlacementDecisionConfigMap(configMapName string) *Actions {
	a.context.t.Helper()

	fixtureClient := utils.GetE2EFixtureK8sClient()

	_, err := fixtureClient.KubeClientset.CoreV1().ConfigMaps(utils.ArgoCDNamespace).Get(context.Background(), configMapName, metav1.GetOptions{})

	// Don't do anything if it exists
	if err == nil {
		return a
	}

	_, err = fixtureClient.KubeClientset.CoreV1().ConfigMaps(utils.ArgoCDNamespace).Create(context.Background(),
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: configMapName,
			},
			Data: map[string]string{
				"apiVersion":    "cluster.open-cluster-management.io/v1alpha1",
				"kind":          "placementdecisions",
				"statusListKey": "decisions",
				"matchKey":      "clusterName",
			},
		}, metav1.CreateOptions{})

	a.describeAction = fmt.Sprintf("creating configmap '%s'", configMapName)
	a.lastOutput, a.lastError = "", err
	a.verifyAction()

	return a
}

func (a *Actions) CreatePlacementDecision(placementDecisionName string) *Actions {
	a.context.t.Helper()

	fixtureClient := utils.GetE2EFixtureK8sClient().DynamicClientset

	_, err := fixtureClient.Resource(pdGVR).Namespace(utils.ArgoCDNamespace).Get(
		context.Background(),
		placementDecisionName,
		metav1.GetOptions{})
	// If already exists
	if err == nil {
		return a
	}

	placementDecision := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      placementDecisionName,
				"namespace": utils.ArgoCDNamespace,
			},
			"kind":       "PlacementDecision",
			"apiVersion": "cluster.open-cluster-management.io/v1alpha1",
			"status":     map[string]interface{}{},
		},
	}

	_, err = fixtureClient.Resource(pdGVR).Namespace(utils.ArgoCDNamespace).Create(
		context.Background(),
		placementDecision,
		metav1.CreateOptions{})

	a.describeAction = fmt.Sprintf("creating placementDecision '%v'", placementDecisionName)
	a.lastOutput, a.lastError = "", err
	a.verifyAction()

	return a
}

func (a *Actions) StatusUpdatePlacementDecision(placementDecisionName string, clusterList []interface{}) *Actions {
	a.context.t.Helper()

	fixtureClient := utils.GetE2EFixtureK8sClient().DynamicClientset
	placementDecision, err := fixtureClient.Resource(pdGVR).Namespace(utils.ArgoCDNamespace).Get(
		context.Background(),
		placementDecisionName,
		metav1.GetOptions{})

	placementDecision.Object["status"] = map[string]interface{}{
		"decisions": clusterList,
	}

	if err == nil {
		_, err = fixtureClient.Resource(pdGVR).Namespace(utils.ArgoCDNamespace).UpdateStatus(
			context.Background(),
			placementDecision,
			metav1.UpdateOptions{})
	}
	a.describeAction = fmt.Sprintf("status update placementDecision for '%v'", clusterList)
	a.lastOutput, a.lastError = "", err
	a.verifyAction()

	return a
}

// Delete deletes the ApplicationSet within the context
func (a *Actions) Delete() *Actions {
	a.context.t.Helper()

	fixtureClient := utils.GetE2EFixtureK8sClient()

	deleteProp := metav1.DeletePropagationForeground
	err := fixtureClient.AppSetClientset.Delete(context.Background(), a.context.name, metav1.DeleteOptions{PropagationPolicy: &deleteProp})
	a.describeAction = fmt.Sprintf("Deleting ApplicationSet '%s' %v", a.context.name, err)
	a.lastOutput, a.lastError = "", err
	a.verifyAction()

	return a
}

// get retrieves the ApplicationSet (by name) that was created by an earlier Create action
func (a *Actions) get() (*v1alpha1.ApplicationSet, error) {
	appSet := v1alpha1.ApplicationSet{}

	fixtureClient := utils.GetE2EFixtureK8sClient()
	newResource, err := fixtureClient.AppSetClientset.Get(context.Background(), a.context.name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	bytes, err := newResource.MarshalJSON()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(bytes, &appSet)
	if err != nil {
		return nil, err
	}

	return &appSet, nil

}

// Update retrieves the latest copy the ApplicationSet, then allows the caller to mutate it via 'toUpdate', with
// the result applied back to the cluster resource
func (a *Actions) Update(toUpdate func(*v1alpha1.ApplicationSet)) *Actions {
	a.context.t.Helper()

	timeout := 30 * time.Second

	var mostRecentError error

	for start := time.Now(); time.Since(start) < timeout; time.Sleep(3 * time.Second) {

		appSet, err := a.get()
		mostRecentError = err
		if err == nil {
			// Keep trying to update until it succeeds, or the test times out
			toUpdate(appSet)
			a.describeAction = fmt.Sprintf("updating ApplicationSet '%s'", appSet.Name)

			fixtureClient := utils.GetE2EFixtureK8sClient()
			_, err = fixtureClient.AppSetClientset.Update(context.Background(), utils.MustToUnstructured(&appSet), metav1.UpdateOptions{})

			if err != nil {
				mostRecentError = err
			} else {
				mostRecentError = nil
				break
			}
		}
	}

	a.lastOutput, a.lastError = "", mostRecentError
	a.verifyAction()

	return a
}

func (a *Actions) verifyAction() {
	a.context.t.Helper()

	if a.describeAction != "" {
		log.Infof("action: %s", a.describeAction)
		a.describeAction = ""
	}

	if !a.ignoreErrors {
		a.Then().Expect(Success(""))
	}

}
