package applicationsets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"

	"github.com/argoproj/argo-cd/v2/common"
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

func (a *Actions) SwitchToExternalNamespace(namespace utils.ExternalNamespace) *Actions {
	a.context.switchToNamespace = namespace
	log.Infof("switched to external namespace: %s", namespace)
	return a
}

func (a *Actions) SwitchToArgoCDNamespace() *Actions {
	a.context.switchToNamespace = ""
	log.Infof("switched to argocd namespace: %s", utils.ArgoCDNamespace)
	return a
}

// CreateClusterSecret creates a faux cluster secret, with the given cluster server and cluster name (this cluster
// will not actually be used by the Argo CD controller, but that's not needed for our E2E tests)
func (a *Actions) CreateClusterSecret(secretName string, clusterName string, clusterServer string) *Actions {
	fixtureClient := utils.GetE2EFixtureK8sClient()

	var serviceAccountName string

	// Look for a service account matching '*application-controller*'
	err := wait.PollUntilContextTimeout(context.Background(), 500*time.Millisecond, 30*time.Second, false, func(ctx context.Context) (bool, error) {
		serviceAccountList, err := fixtureClient.KubeClientset.CoreV1().ServiceAccounts(fixture.TestNamespace()).List(ctx, metav1.ListOptions{})
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
		bearerToken, err = clusterauth.GetServiceAccountBearerToken(fixtureClient.KubeClientset, fixture.TestNamespace(), serviceAccountName, common.BearerTokenTimeout)

		// bearerToken
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: fixture.TestNamespace(),
				Labels: map[string]string{
					common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
					utils.TestingLabel:        "true",
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
	err := utils.GetE2EFixtureK8sClient().KubeClientset.CoreV1().Secrets(fixture.TestNamespace()).Delete(context.Background(), secretName, metav1.DeleteOptions{})

	a.describeAction = fmt.Sprintf("deleting cluster Secret '%s'", secretName)
	a.lastOutput, a.lastError = "", err
	a.verifyAction()

	return a
}

// DeleteConfigMap deletes a faux cluster secret
func (a *Actions) DeleteConfigMap(configMapName string) *Actions {
	err := utils.GetE2EFixtureK8sClient().KubeClientset.CoreV1().ConfigMaps(fixture.TestNamespace()).Delete(context.Background(), configMapName, metav1.DeleteOptions{})

	a.describeAction = fmt.Sprintf("deleting configMap '%s'", configMapName)
	a.lastOutput, a.lastError = "", err
	a.verifyAction()

	return a
}

// DeletePlacementDecision deletes a faux cluster secret
func (a *Actions) DeletePlacementDecision(placementDecisionName string) *Actions {
	err := utils.GetE2EFixtureK8sClient().DynamicClientset.Resource(pdGVR).Namespace(fixture.TestNamespace()).Delete(context.Background(), placementDecisionName, metav1.DeleteOptions{})

	a.describeAction = fmt.Sprintf("deleting placement decision '%s'", placementDecisionName)
	a.lastOutput, a.lastError = "", err
	a.verifyAction()

	return a
}

// Create a temporary namespace, from utils.ApplicationSet, for use by the test.
// This namespace will be deleted on subsequent tests.
func (a *Actions) CreateNamespace(namespace string) *Actions {
	a.context.t.Helper()

	fixtureClient := utils.GetE2EFixtureK8sClient()

	_, err := fixtureClient.KubeClientset.CoreV1().Namespaces().Create(context.Background(),
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{})

	a.describeAction = fmt.Sprintf("creating namespace '%s'", namespace)
	a.lastOutput, a.lastError = "", err
	a.verifyAction()

	return a
}

// Create creates an ApplicationSet using the provided value
func (a *Actions) Create(appSet v1alpha1.ApplicationSet) *Actions {
	a.context.t.Helper()

	fixtureClient := utils.GetE2EFixtureK8sClient()

	appSet.APIVersion = "argoproj.io/v1alpha1"
	appSet.Kind = "ApplicationSet"

	var appSetClientSet dynamic.ResourceInterface

	if a.context.switchToNamespace != "" {
		externalAppSetClientset, found := fixtureClient.ExternalAppSetClientsets[utils.ExternalNamespace(a.context.switchToNamespace)]
		if !found {
			a.lastOutput, a.lastError = "", fmt.Errorf("No external clientset found for %s", a.context.switchToNamespace)
			return a
		}
		appSetClientSet = externalAppSetClientset
	} else {
		appSetClientSet = fixtureClient.AppSetClientset
	}

	newResource, err := appSetClientSet.Create(context.Background(), utils.MustToUnstructured(&appSet), metav1.CreateOptions{})

	if err == nil {
		a.context.name = newResource.GetName()
		a.context.namespace = newResource.GetNamespace()
	}

	a.describeAction = fmt.Sprintf("creating ApplicationSet '%s/%s'", appSet.Namespace, appSet.Name)
	a.lastOutput, a.lastError = "", err
	a.verifyAction()

	return a
}

// Create Role/RoleBinding to allow ApplicationSet to list the PlacementDecisions
func (a *Actions) CreatePlacementRoleAndRoleBinding() *Actions {
	fixtureClient := utils.GetE2EFixtureK8sClient()

	var err error

	_, err = fixtureClient.KubeClientset.RbacV1().Roles(fixture.TestNamespace()).Create(context.Background(), &v1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "placement-role", Namespace: fixture.TestNamespace()},
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
		_, err = fixtureClient.KubeClientset.RbacV1().RoleBindings(fixture.TestNamespace()).Create(context.Background(),
			&v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "placement-role-binding", Namespace: fixture.TestNamespace()},
				Subjects: []v1.Subject{
					{
						Name:      "argocd-applicationset-controller",
						Namespace: fixture.TestNamespace(),
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

	_, err := fixtureClient.KubeClientset.CoreV1().ConfigMaps(fixture.TestNamespace()).Get(context.Background(), configMapName, metav1.GetOptions{})

	// Don't do anything if it exists
	if err == nil {
		return a
	}

	_, err = fixtureClient.KubeClientset.CoreV1().ConfigMaps(fixture.TestNamespace()).Create(context.Background(),
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

	_, err := fixtureClient.Resource(pdGVR).Namespace(fixture.TestNamespace()).Get(
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
				"namespace": fixture.TestNamespace(),
			},
			"kind":       "PlacementDecision",
			"apiVersion": "cluster.open-cluster-management.io/v1alpha1",
			"status":     map[string]interface{}{},
		},
	}

	_, err = fixtureClient.Resource(pdGVR).Namespace(fixture.TestNamespace()).Create(
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
	placementDecision, err := fixtureClient.Resource(pdGVR).Namespace(fixture.TestNamespace()).Get(
		context.Background(),
		placementDecisionName,
		metav1.GetOptions{})

	placementDecision.Object["status"] = map[string]interface{}{
		"decisions": clusterList,
	}

	if err == nil {
		_, err = fixtureClient.Resource(pdGVR).Namespace(fixture.TestNamespace()).UpdateStatus(
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

	var appSetClientSet dynamic.ResourceInterface

	if a.context.switchToNamespace != "" {
		externalAppSetClientset, found := fixtureClient.ExternalAppSetClientsets[utils.ExternalNamespace(a.context.switchToNamespace)]
		if !found {
			a.lastOutput, a.lastError = "", fmt.Errorf("No external clientset found for %s", a.context.switchToNamespace)
			return a
		}
		appSetClientSet = externalAppSetClientset
	} else {
		appSetClientSet = fixtureClient.AppSetClientset
	}

	deleteProp := metav1.DeletePropagationForeground
	err := appSetClientSet.Delete(context.Background(), a.context.name, metav1.DeleteOptions{PropagationPolicy: &deleteProp})
	a.describeAction = fmt.Sprintf("Deleting ApplicationSet '%s/%s' %v", a.context.namespace, a.context.name, err)
	a.lastOutput, a.lastError = "", err
	a.verifyAction()

	return a
}

// get retrieves the ApplicationSet (by name) that was created by an earlier Create action
func (a *Actions) get() (*v1alpha1.ApplicationSet, error) {
	appSet := v1alpha1.ApplicationSet{}

	fixtureClient := utils.GetE2EFixtureK8sClient()

	var appSetClientSet dynamic.ResourceInterface

	if a.context.switchToNamespace != "" {
		externalAppSetClientset, found := fixtureClient.ExternalAppSetClientsets[utils.ExternalNamespace(a.context.switchToNamespace)]
		if !found {
			return nil, fmt.Errorf("No external clientset found for %s", a.context.switchToNamespace)
		}
		appSetClientSet = externalAppSetClientset
	} else {
		appSetClientSet = fixtureClient.AppSetClientset
	}

	newResource, err := appSetClientSet.Get(context.Background(), a.context.name, metav1.GetOptions{})
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
			a.describeAction = fmt.Sprintf("updating ApplicationSet '%s/%s'", appSet.Namespace, appSet.Name)

			fixtureClient := utils.GetE2EFixtureK8sClient()

			var appSetClientSet dynamic.ResourceInterface

			if a.context.switchToNamespace != "" {
				externalAppSetClientset, found := fixtureClient.ExternalAppSetClientsets[utils.ExternalNamespace(a.context.switchToNamespace)]
				if !found {
					a.lastOutput, a.lastError = "", fmt.Errorf("No external clientset found for %s", a.context.switchToNamespace)
					return a
				}
				appSetClientSet = externalAppSetClientset
			} else {
				appSetClientSet = fixtureClient.AppSetClientset
			}

			_, err = appSetClientSet.Update(context.Background(), utils.MustToUnstructured(&appSet), metav1.UpdateOptions{})

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

func (a *Actions) AppSet(appName string, flags ...string) *Actions {
	a.context.t.Helper()
	args := []string{"app", "set", appName}
	args = append(args, flags...)
	a.runCli(args...)
	return a
}

func (a *Actions) runCli(args ...string) {
	a.context.t.Helper()
	a.lastOutput, a.lastError = fixture.RunCli(args...)
	a.verifyAction()
}

func (a *Actions) AddSignedFile(fileName, fileContents string) *Actions {
	a.context.t.Helper()
	fixture.AddSignedFile(a.context.path+"/"+fileName, fileContents)
	return a
}
