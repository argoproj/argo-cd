package controller

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/stevenle/topsort"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/glob"
)

// buildDependencyGraph recursively builds the dependency graph for application
// app.
func (mgr *appStateManager) buildDependencyGraph(app *v1alpha1.Application, depGraph *topsort.Graph) error {
	if app.Spec.DependsOn == nil {
		return nil
	}
	appName := app.GetName()
	logCtx := log.
		WithField("application", app.QualifiedName())

	logCtx.Debugf("calculating dependencies")

	for _, dep := range app.Spec.DependsOn.Selectors {
		// lsel := labels.
		var lsel labels.Selector = labels.Everything()
		var err error
		if dep.LabelSelector != nil {
			lsel, err = v1.LabelSelectorAsSelector(dep.LabelSelector)
			if err != nil {
				return fmt.Errorf("could not convert to label selector: %w", err)
			}
		}
		apps, err := mgr.appLister.Applications(app.GetNamespace()).List(lsel)
		if err != nil {
			return fmt.Errorf("could not retrieve dependencies: %w", err)
		}
		for _, a := range apps {
			// We need to filter out apps belonging to a different project
			if a.Spec.Project != app.Spec.Project {
				logCtx.Infof("cannot depend on application '%s', projects mismatch", a.QualifiedName())
				continue
			}

			if len(dep.NamePattern) > 0 {
				if !glob.MatchStringInList(dep.NamePattern, a.GetName(), false) {
					continue
				}
			}

			// If we already have resolved this app's dependencies, no need to
			// get them again and potentially inflict an endless recursion.
			if depGraph.ContainsNode(a.GetName()) {
				err = depGraph.AddEdge(appName, a.GetName())
				if err != nil {
					return fmt.Errorf("could not calculate dependencies: %w", err)
				}
				continue
			}

			// Store the dependency
			err = depGraph.AddEdge(appName, a.GetName())
			if err != nil {
				return fmt.Errorf("could not calculate dependencies: %w", err)
			}
			err = mgr.buildDependencyGraph(a, depGraph)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// ResolveApplicationDependencies recursively resolves all dependencies of the
// given application and returns a topologically sorted list of application
// names.
func (mgr *appStateManager) ResolveApplicationDependencies(app *v1alpha1.Application) ([]string, error) {
	if app.Spec.DependsOn == nil || len(app.Spec.DependsOn.Selectors) == 0 {
		return nil, nil
	}

	depGraph := topsort.NewGraph()
	err := mgr.buildDependencyGraph(app, depGraph)
	if err != nil {
		return nil, err
	}
	resolved, err := depGraph.TopSort(app.GetName())
	// We remove ourselves from the dependency list before returning it
	if len(resolved) > 0 {
		return resolved[:len(resolved)-1], err
	} else {
		return resolved, err
	}
}
