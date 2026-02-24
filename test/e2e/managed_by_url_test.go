package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/settings"
	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

const managedByURLTestPath = "guestbook"

// TestManagedByURLWithAnnotation tests that applications with managed-by-url annotation
// include the managed-by-url in their deep links
func TestManagedByURLWithAnnotation(t *testing.T) {
	managedByURL := "https://argocd-instance-b.example.com"

	ctx := Given(t)
	ctx.
		Project("default").
		Path(managedByURLTestPath).
		When().
		CreateApp().
		And(func() {
			// Add managed-by-url annotation to the application with retry logic
			for i := range 3 {
				appObj, err := fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Get(t.Context(), ctx.GetName(), metav1.GetOptions{})
				require.NoError(t, err)

				if appObj.Annotations == nil {
					appObj.Annotations = make(map[string]string)
				}
				appObj.Annotations[AnnotationKeyManagedByURL] = managedByURL

				_, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Update(t.Context(), appObj, metav1.UpdateOptions{})
				if err == nil {
					break
				}
				if i == 2 {
					require.NoError(t, err)
				}
				time.Sleep(100 * time.Millisecond)
			}
		}).
		And(func() {
			// Configure deep links that use the managed-by-url template variable
			deepLinksConfig := `- url: "{{.managedByURL}}/applications/{{.app.metadata.name}}"
  title: "Managed By Instance"
  description: "Open in managing ArgoCD instance"`

			// Update the argocd-cm configmap to include our test deep links and URL
			configMap, err := fixture.KubeClientset.CoreV1().ConfigMaps(fixture.ArgoCDNamespace).Get(t.Context(), "argocd-cm", metav1.GetOptions{})
			require.NoError(t, err)

			if configMap.Data == nil {
				configMap.Data = make(map[string]string)
			}
			configMap.Data["application.links"] = deepLinksConfig
			configMap.Data["url"] = "https://argocd-test.example.com"

			_, err = fixture.KubeClientset.CoreV1().ConfigMaps(fixture.ArgoCDNamespace).Update(t.Context(), configMap, metav1.UpdateOptions{})
			require.NoError(t, err)

			// Wait a moment for the config to be picked up
			time.Sleep(2 * time.Second)
		}).
		Then().
		And(func(app *Application) {
			// Test that the managed-by-url annotation is preserved
			assert.Equal(t, managedByURL, app.Annotations[AnnotationKeyManagedByURL])

			// Test that the application links include the managed-by-url in the deep links
			conn, appClient, err := fixture.ArgoCDClientset.NewApplicationClient()
			require.NoError(t, err)
			defer conn.Close()

			links, err := appClient.ListLinks(t.Context(), &application.ListAppLinksRequest{
				Name: ptr.To(app.Name),
			})
			require.NoError(t, err)

			// Verify that deep links are generated with the managed-by-url
			assert.NotNil(t, links)
			assert.Len(t, links.Items, 1, "Should have 1 deep link configured")

			// Check that the link uses the managed-by-url
			expectedLink := managedByURL + "/applications/" + app.Name
			found := false
			for _, link := range links.Items {
				if link.Url != nil && *link.Url == expectedLink {
					found = true
					assert.Equal(t, "Managed By Instance", *link.Title)
					assert.Equal(t, "Open in managing ArgoCD instance", *link.Description)
					break
				}
			}
			assert.True(t, found, "Deep link should use managed-by-url: %s", expectedLink)
		}).
		And(func(_ *Application) {
			// Clean up: remove the test deep links configuration
			configMap, err := fixture.KubeClientset.CoreV1().ConfigMaps(fixture.ArgoCDNamespace).Get(t.Context(), "argocd-cm", metav1.GetOptions{})
			if err == nil && configMap.Data != nil {
				delete(configMap.Data, "application.links")
				delete(configMap.Data, "url")
				_, err := fixture.KubeClientset.CoreV1().ConfigMaps(fixture.ArgoCDNamespace).Update(t.Context(), configMap, metav1.UpdateOptions{})
				if err != nil {
					t.Logf("Failed to clean up configmap: %v", err)
				}
			}
		})
}

// TestManagedByURLFallbackToCurrentInstance tests that when no managed-by-url is set,
// the current instance URL is used as fallback
func TestManagedByURLFallbackToCurrentInstance(t *testing.T) {
	Given(t).
		Project("default").
		Path(managedByURLTestPath).
		When().
		CreateApp().
		And(func() {
			// Configure deep links that use the managed-by-url template variable
			deepLinksConfig := `- url: "{{.managedByURL}}/applications/{{.app.metadata.name}}"
  title: "Managed By Instance"
  description: "Open in managing ArgoCD instance"`

			// Update the argocd-cm configmap to include our test deep links and URL
			configMap, err := fixture.KubeClientset.CoreV1().ConfigMaps(fixture.ArgoCDNamespace).Get(t.Context(), "argocd-cm", metav1.GetOptions{})
			require.NoError(t, err)

			if configMap.Data == nil {
				configMap.Data = make(map[string]string)
			}
			configMap.Data["application.links"] = deepLinksConfig
			configMap.Data["url"] = "https://argocd-test.example.com"

			_, err = fixture.KubeClientset.CoreV1().ConfigMaps(fixture.ArgoCDNamespace).Update(t.Context(), configMap, metav1.UpdateOptions{})
			require.NoError(t, err)

			// Wait a moment for the config to be picked up
			time.Sleep(2 * time.Second)
		}).
		Then().
		And(func(app *Application) {
			// Test that the application links use the current instance URL as fallback
			conn, appClient, err := fixture.ArgoCDClientset.NewApplicationClient()
			require.NoError(t, err)
			defer conn.Close()

			links, err := appClient.ListLinks(t.Context(), &application.ListAppLinksRequest{
				Name: ptr.To(app.Name),
			})
			require.NoError(t, err)

			// Verify that deep links are generated with the current instance URL as fallback
			assert.NotNil(t, links)
			assert.Len(t, links.Items, 1, "Should have 1 deep link configured")

			// Get the current ArgoCD server URL from settings
			conn2, settingsClient, err := fixture.ArgoCDClientset.NewSettingsClient()
			require.NoError(t, err)
			defer conn2.Close()

			settings, err := settingsClient.Get(t.Context(), &settings.SettingsQuery{})
			require.NoError(t, err)

			// Check that the link uses the current instance URL as fallback
			expectedLink := settings.URL + "/applications/" + app.Name
			found := false
			for _, link := range links.Items {
				if link.Url != nil && *link.Url == expectedLink {
					found = true
					assert.Equal(t, "Managed By Instance", *link.Title)
					assert.Equal(t, "Open in managing ArgoCD instance", *link.Description)
					break
				}
			}
			if !found {
				t.Logf("Returned links:")
				for _, link := range links.Items {
					if link.Url != nil {
						t.Logf("- %s", *link.Url)
					}
				}
			}
			assert.True(t, found, "Deep link should use current instance URL as fallback: %s", expectedLink)
		}).
		And(func(_ *Application) {
			// Clean up: remove the test deep links configuration
			configMap, err := fixture.KubeClientset.CoreV1().ConfigMaps(fixture.ArgoCDNamespace).Get(t.Context(), "argocd-cm", metav1.GetOptions{})
			if err == nil && configMap.Data != nil {
				delete(configMap.Data, "application.links")
				delete(configMap.Data, "url")
				_, err := fixture.KubeClientset.CoreV1().ConfigMaps(fixture.ArgoCDNamespace).Update(t.Context(), configMap, metav1.UpdateOptions{})
				if err != nil {
					t.Logf("Failed to clean up configmap: %v", err)
				}
			}
		})
}
