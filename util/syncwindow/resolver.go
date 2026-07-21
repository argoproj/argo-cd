package syncwindow

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	listers "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
)

// Resolver resolves SyncWindowRef references into concrete SyncWindow objects
// that can be evaluated by the existing sync window logic.
type Resolver struct {
	lister    listers.SyncWindowResourceLister
	namespace string
}

// NewResolver creates a new Resolver with the given lister and namespace.
func NewResolver(lister listers.SyncWindowResourceLister, namespace string) *Resolver {
	return &Resolver{
		lister:    lister,
		namespace: namespace,
	}
}

// ResolveProjectRefs resolves SyncWindowProjectRef entries from an AppProject into SyncWindow objects.
// The returned windows incorporate any application/namespace/cluster filters from the project ref.
func (r *Resolver) ResolveProjectRefs(refs []v1alpha1.SyncWindowProjectRef) (v1alpha1.SyncWindows, error) {
	var result v1alpha1.SyncWindows
	for _, ref := range refs {
		resources, err := r.resolveRef(ref.Ref)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve sync window ref in project: %w", err)
		}
		for _, res := range resources {
			for i := range res.Spec.Windows {
				def := &res.Spec.Windows[i]
				sw := definitionToSyncWindow(def)
				// If the project ref specifies filters, override the ones from the definition
				if len(ref.Applications) > 0 {
					sw.Applications = ref.Applications
				}
				if len(ref.Namespaces) > 0 {
					sw.Namespaces = ref.Namespaces
				}
				if len(ref.Clusters) > 0 {
					sw.Clusters = ref.Clusters
				}
				result = append(result, sw)
			}
		}
	}
	return result, nil
}

// ResolveAppRefs resolves SyncWindowRef entries from an Application into SyncWindow objects.
// The returned windows have their application/namespace/cluster filters cleared since
// they apply directly to the referencing application.
func (r *Resolver) ResolveAppRefs(refs []v1alpha1.SyncWindowRef) (v1alpha1.SyncWindows, error) {
	var result v1alpha1.SyncWindows
	for _, ref := range refs {
		resources, err := r.resolveRef(ref)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve sync window ref in application: %w", err)
		}
		for _, res := range resources {
			for i := range res.Spec.Windows {
				def := &res.Spec.Windows[i]
				sw := definitionToSyncWindow(def)
				// Clear filters - when referenced from an app, the window applies directly
				sw.Applications = nil
				sw.Namespaces = nil
				sw.Clusters = nil
				result = append(result, sw)
			}
		}
	}
	return result, nil
}

// resolveRef resolves a single SyncWindowRef to a list of SyncWindowResource objects.
func (r *Resolver) resolveRef(ref v1alpha1.SyncWindowRef) ([]*v1alpha1.SyncWindowResource, error) {
	if ref.Name != "" {
		sw, err := r.lister.SyncWindowResources(r.namespace).Get(ref.Name)
		if err != nil {
			return nil, fmt.Errorf("sync window resource %q not found: %w", ref.Name, err)
		}
		return []*v1alpha1.SyncWindowResource{sw}, nil
	}
	if ref.Selector != nil {
		selector, err := convertLabelSelector(ref.Selector)
		if err != nil {
			return nil, fmt.Errorf("invalid label selector: %w", err)
		}
		return r.lister.SyncWindowResources(r.namespace).List(selector)
	}
	return nil, fmt.Errorf("sync window ref must specify either name or selector")
}

// convertLabelSelector converts a metav1.LabelSelector to a labels.Selector.
func convertLabelSelector(ls *metav1.LabelSelector) (labels.Selector, error) {
	return metav1.LabelSelectorAsSelector(ls)
}

// definitionToSyncWindow converts a SyncWindowDefinition from a CRD into the existing SyncWindow type.
func definitionToSyncWindow(def *v1alpha1.SyncWindowDefinition) *v1alpha1.SyncWindow {
	return &v1alpha1.SyncWindow{
		Kind:           def.Kind,
		Schedule:       def.Schedule,
		Duration:       def.Duration,
		Applications:   def.Applications,
		Namespaces:     def.Namespaces,
		Clusters:       def.Clusters,
		ManualSync:     def.ManualSync,
		TimeZone:       def.TimeZone,
		UseAndOperator: def.UseAndOperator,
		Description:    def.Description,
		SyncOverrun:    def.SyncOverrun,
	}
}
