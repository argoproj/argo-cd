package argo

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/argoproj/argo-cd/common"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	testcore "k8s.io/client-go/testing"
)

func TestRefreshApp(t *testing.T) {
	var testApp argoappv1.Application
	testApp.Name = "test-app"
	testApp.Namespace = "default"
	appClientset := appclientset.NewSimpleClientset(&testApp)
	appIf := appClientset.ArgoprojV1alpha1().Applications("default")
	_, err := RefreshApp(appIf, "test-app")
	assert.Nil(t, err)
	// For some reason, the fake Application inferface doesn't reflect the patch status after Patch(),
	// so can't verify it was set in unit tests.
	//_, ok := newApp.Annotations[common.AnnotationKeyRefresh]
	//assert.True(t, ok)
}

func TestWaitForRefresh(t *testing.T) {
	appClientset := appclientset.NewSimpleClientset()

	// Verify timeout
	appIf := appClientset.ArgoprojV1alpha1().Applications("default")
	oneHundredMs := 100 * time.Millisecond
	app, err := WaitForRefresh(appIf, "test-app", &oneHundredMs)
	assert.NotNil(t, err)
	assert.Nil(t, app)
	assert.Contains(t, strings.ToLower(err.Error()), "timed out")

	// Verify success
	var testApp argoappv1.Application
	testApp.Name = "test-app"
	testApp.Namespace = "default"
	testApp.ObjectMeta.Annotations = map[string]string{
		common.AnnotationKeyRefresh: time.Now().UTC().Format(time.RFC3339),
	}
	testApp.Status.ComparisonResult.ComparedAt = metav1.Time{Time: time.Now().UTC()}
	appClientset = appclientset.NewSimpleClientset()

	appIf = appClientset.ArgoprojV1alpha1().Applications("default")
	watcher := watch.NewFake()
	appClientset.PrependWatchReactor("applications", testcore.DefaultWatchReactor(watcher, nil))
	// simulate add/update/delete watch events
	go watcher.Add(&testApp)
	app, err = WaitForRefresh(appIf, "test-app", &oneHundredMs)
	assert.Nil(t, err)
	assert.NotNil(t, app)
}
