package mocks

import (
	"context"
	"errors"

	"github.com/argoproj/argo-cd/v3/util/settings"

	corev1 "k8s.io/api/core/v1"
	informersv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

// This mocks just enough of ArgoDBSettingsSource to test whether or not the DB
// cluster methods call GetSettings or not.

type GetSettingsCounterMock struct {
	ns string
	clientset kubernetes.Interface
	getSettingsCallCount int
}

func NewGetSettingsCounterMock(ns string, clientset kubernetes.Interface) *GetSettingsCounterMock {
	return &GetSettingsCounterMock {
		ns: ns,
		clientset: clientset,
	}
}

func (mgr *GetSettingsCounterMock) GetGetSettingsCallCount() int {
	return mgr.getSettingsCallCount
}

func (mgr *GetSettingsCounterMock) GetSettings() (*settings.ArgoCDSettings, error) {
	mgr.getSettingsCallCount++

	return nil, errors.New("not implemented")
}

func (mgr *GetSettingsCounterMock) SaveSSHKnownHostsData(context.Context, []string) error {
	return errors.New("not implemented")
}

func (mgr *GetSettingsCounterMock) SaveTLSCertificateData(context.Context, map[string]string) error {
	return errors.New("not implemented")
}

func (mgr *GetSettingsCounterMock) GetConfigMapByName(string) (*corev1.ConfigMap, error) {
	return nil, errors.New("not implemented")
}

func (mgr *GetSettingsCounterMock) ResyncInformers() error {
	return errors.New("not implemented")
}

func (mgr *GetSettingsCounterMock) GetSecretsInformer() (cache.SharedIndexInformer, error) {
	nilIndexer := func(obj any) ([]string, error) {
		return nil, nil
	}

	indexers := cache.Indexers{
		"byClusterURL": nilIndexer,
		"byClusterName": nilIndexer,
		"byProjectCluster": nilIndexer,
		"byProjectRepo": nilIndexer,
		"byProjectRepoWrite": nilIndexer,
	}

	return informersv1.NewSecretInformer(mgr.clientset, mgr.ns, 0, indexers), nil
}

func (mgr *GetSettingsCounterMock) GetSecretByName(string) (*corev1.Secret, error) {
	return nil, errors.New("not implemented")
}

func (mgr *GetSettingsCounterMock) GetNamespace() string {
	return mgr.ns
}

func (mgr *GetSettingsCounterMock) SaveGPGPublicKeyData(context.Context, map[string]string) error {
	return errors.New("not implemented")
}

func (mgr *GetSettingsCounterMock) GetSecretsLister() (v1listers.SecretLister, error) {
	return nil, errors.New("not implemented")
}
