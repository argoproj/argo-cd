package configbus

import (
	"fmt"
	"time"

	"sigs.k8s.io/yaml"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// configurationHolder is satisfied by InformerCRDSource and StaticCRDSource.
type configurationHolder interface {
	get() *appv1.ArgoCDConfiguration
}

func (s *InformerCRDSource) Config() *appv1.ArgoCDConfiguration { return s.get() }
func (s StaticCRDSource) Config() *appv1.ArgoCDConfiguration    { return s.get() }

func (s *InformerCRDSource) HasReconciliationTimeout() bool {
	return hasReconciliationTimeout(s)
}

func (s *InformerCRDSource) ReconciliationTimeout() time.Duration {
	return reconciliationTimeout(s)
}

func (s *InformerCRDSource) HasHardReconciliationTimeout() bool {
	return hasHardReconciliationTimeout(s)
}

func (s *InformerCRDSource) HardReconciliationTimeout() time.Duration {
	return hardReconciliationTimeout(s)
}

func (s *InformerCRDSource) HasReconciliationJitter() bool {
	return hasReconciliationJitter(s)
}

func (s *InformerCRDSource) ReconciliationJitter() time.Duration {
	return reconciliationJitter(s)
}

func (s *InformerCRDSource) HasResourceOverrides() bool {
	return hasResourceOverrides(s)
}

func (s *InformerCRDSource) ResourceOverrides() (map[string]appv1.ResourceOverride, error) {
	return resourceOverrides(s)
}

func (s StaticCRDSource) HasReconciliationTimeout() bool {
	return hasReconciliationTimeout(s)
}

func (s StaticCRDSource) ReconciliationTimeout() time.Duration {
	return reconciliationTimeout(s)
}

func (s StaticCRDSource) HasHardReconciliationTimeout() bool {
	return hasHardReconciliationTimeout(s)
}

func (s StaticCRDSource) HardReconciliationTimeout() time.Duration {
	return hardReconciliationTimeout(s)
}

func (s StaticCRDSource) HasReconciliationJitter() bool {
	return hasReconciliationJitter(s)
}

func (s StaticCRDSource) ReconciliationJitter() time.Duration {
	return reconciliationJitter(s)
}

func (s StaticCRDSource) HasResourceOverrides() bool {
	return hasResourceOverrides(s)
}

func (s StaticCRDSource) ResourceOverrides() (map[string]appv1.ResourceOverride, error) {
	return resourceOverrides(s)
}

func hasReconciliationTimeout(h configurationHolder) bool {
	cfg := h.get()
	return cfg != nil &&
		cfg.Spec.Controller != nil &&
		cfg.Spec.Controller.Reconciliation != nil &&
		cfg.Spec.Controller.Reconciliation.Timeout != nil
}

func reconciliationTimeout(h configurationHolder) time.Duration {
	if !hasReconciliationTimeout(h) {
		return 0
	}
	return h.get().Spec.Controller.Reconciliation.Timeout.Duration
}

func hasHardReconciliationTimeout(h configurationHolder) bool {
	cfg := h.get()
	return cfg != nil &&
		cfg.Spec.Controller != nil &&
		cfg.Spec.Controller.Reconciliation != nil &&
		cfg.Spec.Controller.Reconciliation.HardTimeout != nil
}

func hardReconciliationTimeout(h configurationHolder) time.Duration {
	if !hasHardReconciliationTimeout(h) {
		return 0
	}
	return h.get().Spec.Controller.Reconciliation.HardTimeout.Duration
}

func hasReconciliationJitter(h configurationHolder) bool {
	cfg := h.get()
	return cfg != nil &&
		cfg.Spec.Controller != nil &&
		cfg.Spec.Controller.Reconciliation != nil &&
		cfg.Spec.Controller.Reconciliation.Jitter != nil
}

func reconciliationJitter(h configurationHolder) time.Duration {
	if !hasReconciliationJitter(h) {
		return 0
	}
	return h.get().Spec.Controller.Reconciliation.Jitter.Duration
}

func hasResourceOverrides(h configurationHolder) bool {
	cfg := h.get()
	if cfg == nil || cfg.Spec.Controller == nil || cfg.Spec.Controller.Resource == nil {
		return false
	}
	r := cfg.Spec.Controller.Resource
	return len(r.Health) > 0 ||
		len(r.Actions) > 0 ||
		len(r.IgnoreDifferences) > 0 ||
		len(r.IgnoreResourceUpdates) > 0 ||
		len(r.KnownTypeFields) > 0
}

func resourceOverrides(h configurationHolder) (map[string]appv1.ResourceOverride, error) {
	if !hasResourceOverrides(h) {
		return nil, nil
	}
	return mergeResourceOverrides(h.get().Spec.Controller.Resource)
}

func mergeResourceOverrides(r *appv1.ResourceConfig) (map[string]appv1.ResourceOverride, error) {
	out := map[string]appv1.ResourceOverride{}
	ensure := func(group, kind string) appv1.ResourceOverride {
		key := resourceOverrideKey(group, kind)
		if v, ok := out[key]; ok {
			return v
		}
		return appv1.ResourceOverride{}
	}
	put := func(group, kind string, v appv1.ResourceOverride) {
		out[resourceOverrideKey(group, kind)] = v
	}

	for _, c := range r.Health {
		v := ensure(c.Group, c.Kind)
		v.HealthLua = c.HealthLua
		v.UseOpenLibs = c.UseOpenLibs
		put(c.Group, c.Kind, v)
	}
	for _, c := range r.Actions {
		blob, err := marshalResourceActions(c)
		if err != nil {
			return nil, fmt.Errorf("marshal actions for %s/%s: %w", c.Group, c.Kind, err)
		}
		if blob == "" {
			continue
		}
		v := ensure(c.Group, c.Kind)
		v.Actions = blob
		put(c.Group, c.Kind, v)
	}
	for _, c := range r.IgnoreDifferences {
		v := ensure(c.Group, c.Kind)
		v.IgnoreDifferences = appv1.OverrideIgnoreDiff{
			JSONPointers:          append([]string(nil), c.JSONPointers...),
			JQPathExpressions:     append([]string(nil), c.JQPathExpressions...),
			ManagedFieldsManagers: append([]string(nil), c.ManagedFieldsManagers...),
		}
		put(c.Group, c.Kind, v)
	}
	for _, c := range r.IgnoreResourceUpdates {
		v := ensure(c.Group, c.Kind)
		v.IgnoreResourceUpdates = appv1.OverrideIgnoreDiff{
			JSONPointers:          append([]string(nil), c.JSONPointers...),
			JQPathExpressions:     append([]string(nil), c.JQPathExpressions...),
			ManagedFieldsManagers: append([]string(nil), c.ManagedFieldsManagers...),
		}
		put(c.Group, c.Kind, v)
	}
	for _, c := range r.KnownTypeFields {
		v := ensure(c.Group, c.Kind)
		fields := make([]appv1.KnownTypeField, 0, len(c.Fields))
		for _, f := range c.Fields {
			fields = append(fields, appv1.KnownTypeField{Field: f.Field, Type: f.Type})
		}
		v.KnownTypeFields = fields
		put(c.Group, c.Kind, v)
	}
	return out, nil
}

func resourceOverrideKey(group, kind string) string {
	if group == "*" && kind == "*" {
		return "*/*"
	}
	if group == "" {
		return kind
	}
	return group + "/" + kind
}

func marshalResourceActions(c appv1.ResourceActionsCustomization) (string, error) {
	if c.DiscoveryLua == "" && !c.MergeBuiltinActions && len(c.Definitions) == 0 {
		return "", nil
	}
	m := map[string]any{}
	if c.DiscoveryLua != "" {
		m["discovery.lua"] = c.DiscoveryLua
	}
	if c.MergeBuiltinActions {
		m["mergeBuiltinActions"] = true
	}
	if len(c.Definitions) > 0 {
		defs := make([]any, 0, len(c.Definitions))
		for _, d := range c.Definitions {
			defs = append(defs, map[string]any{
				"name":       d.Name,
				"action.lua": d.ActionLua,
			})
		}
		m["definitions"] = defs
	}
	b, err := yaml.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
