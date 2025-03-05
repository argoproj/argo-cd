package application

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util"
)

func Test_deepCopyApplicationLister_List(t *testing.T) {
	app1 := &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "app1"}}
	app2 := &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "app2"}}
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	_ = indexer.Add(app1)
	_ = indexer.Add(app2)
	lister := &deepCopyApplicationLister{applisters.NewApplicationLister(indexer)}

	tests := []struct {
		name     string
		selector labels.Selector
		expected []*v1alpha1.Application
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name:     "List all applications",
			selector: labels.Everything(),
			expected: []*v1alpha1.Application{app1, app2},
			wantErr:  assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apps, err := lister.List(tt.selector)
			if !tt.wantErr(t, err, fmt.Sprintf("List(%v)", tt.selector)) {
				return
			}
			assert.Equal(t, util.SliceCopy(tt.expected), apps)
		})
	}
}

func Test_deepCopyApplicationLister_Applications(t *testing.T) {
	app1 := &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "app1", Namespace: "default"}}
	app2 := &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "app2", Namespace: "default"}}
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	_ = indexer.Add(app1)
	_ = indexer.Add(app2)
	lister := &deepCopyApplicationLister{applisters.NewApplicationLister(indexer)}

	tests := []struct {
		name      string
		namespace string
		expected  []*v1alpha1.Application
		wantErr   assert.ErrorAssertionFunc
	}{
		{
			name:      "List all applications in namespace",
			namespace: "default",
			expected:  []*v1alpha1.Application{app1, app2},
			wantErr:   assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nsLister := lister.Applications(tt.namespace)
			apps, err := nsLister.List(labels.Everything())
			if !tt.wantErr(t, err, fmt.Sprintf("Applications(%v)", tt.namespace)) {
				return
			}
			assert.Equal(t, util.SliceCopy(tt.expected), apps)
		})
	}
}

func Test_deepCopyApplicationNamespaceLister_List(t *testing.T) {
	app1 := &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "app1", Namespace: "default"}}
	app2 := &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "app2", Namespace: "default"}}
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	_ = indexer.Add(app1)
	_ = indexer.Add(app2)
	lister := &deepCopyApplicationLister{applisters.NewApplicationLister(indexer)}

	tests := []struct {
		name     string
		selector labels.Selector
		expected []*v1alpha1.Application
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name:     "List all applications in namespace",
			selector: labels.Everything(),
			expected: []*v1alpha1.Application{app1, app2},
			wantErr:  assert.NoError,
		},
		{
			name:     "error out",
			selector: labels.Nothing(),
			expected: []*v1alpha1.Application{app1, app2},
			wantErr:  assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apps, err := lister.List(tt.selector)
			if !tt.wantErr(t, err, fmt.Sprintf("List(%v)", tt.selector)) {
				return
			}
			assert.Equal(t, tt.expected, apps)
			assert.NotSame(t, &tt.expected, &apps)
			for i, a := range tt.expected {
				assert.NotSame(t, a, apps[i])
			}
		})
	}
}

func Test_deepCopyApplicationNamespaceLister_Get(t *testing.T) {
	app := &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "app1", Namespace: "default"}}
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	_ = indexer.Add(app)
	lister := &deepCopyApplicationLister{applisters.NewApplicationLister(indexer)}

	tests := []struct {
		name     string
		appName  string
		expected *v1alpha1.Application
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name:     "Get application by name",
			appName:  "app1",
			expected: app,
			wantErr:  assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retrievedApp, err := lister.Applications("default").Get(tt.appName)
			if !tt.wantErr(t, err, fmt.Sprintf("Get(%v)", tt.appName)) {
				return
			}
			assert.Equal(t, tt.expected, retrievedApp)
			assert.NotSame(t, tt.expected, &retrievedApp)
		})
	}
}
