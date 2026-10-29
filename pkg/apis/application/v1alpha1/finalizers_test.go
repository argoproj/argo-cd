package v1alpha1

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/validation"
)

func TestApplicationFinalizerWritersRemainCompatible(t *testing.T) {
	for _, tc := range []struct {
		name     string
		set      func(*Application)
		expected string
	}{
		{"resources", func(app *Application) { app.SetCascadedDeletion(ResourcesFinalizerName) }, "resources-finalizer.argocd.argoproj.io"},
		{"pre-delete", func(app *Application) { app.SetPreDeleteFinalizer() }, "pre-delete-finalizer.argocd.argoproj.io"},
		{"pre-delete cleanup", func(app *Application) { app.SetPreDeleteFinalizer("cleanup") }, "pre-delete-finalizer.argocd.argoproj.io/cleanup"},
		{"post-delete", func(app *Application) { app.SetPostDeleteFinalizer() }, "post-delete-finalizer.argocd.argoproj.io"},
		{"post-delete cleanup", func(app *Application) { app.SetPostDeleteFinalizer("cleanup") }, "post-delete-finalizer.argocd.argoproj.io/cleanup"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := &Application{}
			tc.set(app)
			assert.Equal(t, []string{tc.expected}, app.Finalizers)
		})
	}
}

func TestQualifiedApplicationFinalizersAreValid(t *testing.T) {
	for _, name := range []string{
		QualifiedResourcesFinalizerName,
		QualifiedPreDeleteFinalizerName,
		QualifiedPreDeleteFinalizerName + "-cleanup",
		QualifiedPostDeleteFinalizerName,
		QualifiedPostDeleteFinalizerName + "-cleanup",
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, 1, strings.Count(name, "/"))
			assert.Empty(t, validation.IsQualifiedName(name))
		})
	}
}

func TestApplicationRecognizesResourceFinalizers(t *testing.T) {
	for _, finalizers := range [][]string{
		{"argocd.argoproj.io/resources-finalizer"},
		{"resources-finalizer.argocd.argoproj.io"},
		{"resources-finalizer.argocd.argoproj.io/foreground"},
		{"resources-finalizer.argocd.argoproj.io/background"},
		{"argocd.argoproj.io/resources-finalizer", "resources-finalizer.argocd.argoproj.io"},
		{"resources-finalizer.argocd.argoproj.io", "argocd.argoproj.io/resources-finalizer"},
	} {
		t.Run(strings.Join(finalizers, ","), func(t *testing.T) {
			app := &Application{Finalizers: append([]string{"example.com/keep"}, finalizers...)}
			assert.True(t, app.CascadedDeletion())
			assert.Equal(t, finalizers[0], app.GetPropagationPolicy())
			app.UnSetCascadedDeletion()
			assert.Equal(t, []string{"example.com/keep"}, app.Finalizers)
			assert.False(t, app.CascadedDeletion())
		})
	}
}

func TestApplicationDeleteHookFinalizerCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name      string
		qualified string
		existing  string
		has       func(*Application, ...string) bool
		unset     func(*Application, ...string)
		unsetAll  func(*Application)
	}{
		{"pre-delete", "argocd.argoproj.io/pre-delete-finalizer", "pre-delete-finalizer.argocd.argoproj.io", (*Application).HasPreDeleteFinalizer, (*Application).UnSetPreDeleteFinalizer, (*Application).UnSetPreDeleteFinalizerAll},
		{"post-delete", "argocd.argoproj.io/post-delete-finalizer", "post-delete-finalizer.argocd.argoproj.io", (*Application).HasPostDeleteFinalizer, (*Application).UnSetPostDeleteFinalizer, (*Application).UnSetPostDeleteFinalizerAll},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, stage := range []string{"", "cleanup"} {
				args := []string{}
				qualified, existing := tc.qualified, tc.existing
				if stage != "" {
					args = append(args, stage)
					qualified += "-" + stage
					existing += "/" + stage
				}
				for _, names := range [][]string{{qualified}, {existing}, {qualified, existing}} {
					t.Run(strings.Join(names, ","), func(t *testing.T) {
						app := &Application{Finalizers: append([]string{"example.com/keep"}, names...)}
						assert.True(t, tc.has(app, args...))
						tc.unset(app, args...)
						assert.False(t, tc.has(app, args...))
						assert.Equal(t, []string{"example.com/keep"}, app.Finalizers)
					})
				}
			}
			t.Run("remove every stage", func(t *testing.T) {
				app := &Application{Finalizers: []string{
					tc.qualified, tc.existing, tc.qualified + "-cleanup", tc.existing + "/cleanup",
					"example.com/keep", tc.existing + "-other",
				}}
				tc.unsetAll(app)
				assert.ElementsMatch(t, []string{"example.com/keep", tc.existing + "-other"}, app.Finalizers)
			})
		})
	}
}

func TestAppProjectFinalizerCompatibility(t *testing.T) {
	for _, names := range [][]string{
		{"argocd.argoproj.io/resources-finalizer"},
		{"resources-finalizer.argocd.argoproj.io"},
		{"argocd.argoproj.io/resources-finalizer", "resources-finalizer.argocd.argoproj.io"},
	} {
		t.Run(strings.Join(names, ","), func(t *testing.T) {
			proj := &AppProject{Finalizers: append([]string{"example.com/keep"}, names...)}
			assert.True(t, proj.HasFinalizer())
			proj.RemoveFinalizer()
			assert.False(t, proj.HasFinalizer())
			assert.Equal(t, []string{"example.com/keep"}, proj.Finalizers)
		})
	}
}
