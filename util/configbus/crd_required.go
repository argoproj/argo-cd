package configbus

import (
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// requireCRDField returns the resolved CRD value when present. Nil source,
// nil Config, or !ok → ErrNotConfigured so HybridProvider can fall back.
// The name argument documents the Provider method for reviewers; it is not
// embedded in the error (Hybrid must detect unset via errors.Is).
func requireCRDField[T any](p *CRDProvider, _ string, resolve func(*appv1.ArgoCDConfiguration) (T, bool)) (T, error) {
	var zero T
	cfg, err := p.config()
	if err != nil {
		return zero, err
	}
	v, ok := resolve(cfg)
	if !ok {
		return zero, ErrNotConfigured
	}
	return v, nil
}

// requireCRDFieldErr is like requireCRDField but forwards resolver errors.
func requireCRDFieldErr[T any](p *CRDProvider, _ string, resolve func(*appv1.ArgoCDConfiguration) (T, bool, error)) (T, error) {
	var zero T
	cfg, err := p.config()
	if err != nil {
		return zero, err
	}
	v, ok, err := resolve(cfg)
	if err != nil {
		return zero, err
	}
	if !ok {
		return zero, ErrNotConfigured
	}
	return v, nil
}

func (p *CRDProvider) config() (*appv1.ArgoCDConfiguration, error) {
	if p == nil || p.source == nil {
		return nil, ErrNotConfigured
	}
	cfg := p.source.Config()
	if cfg == nil {
		return nil, ErrNotConfigured
	}
	return cfg, nil
}
