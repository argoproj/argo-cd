package rbac

import (
	"context"
	"fmt"
	application "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/util/rbac"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// Server provides a RBAC service
type Server struct {
	enf           *rbac.Enforcer
	kubeClientset kubernetes.Interface
	namespace     string
	configmap     string
}

// NewServer returns a new instance of the RBAC service
func NewServer(enf *rbac.Enforcer, kubeClientset kubernetes.Interface, namespace string, configmap string) *Server {
	return &Server{enf: enf,
		kubeClientset: kubeClientset,
		namespace:     namespace,
		configmap:     configmap,
	}
}

// fetchConfigMap fetches the configmap or returns the error
func (s *Server) fetchConfigMap(ctx context.Context) (*apiv1.ConfigMap, error) {
	return s.kubeClientset.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.configmap, metav1.GetOptions{})
}

// AddPolicy adds a policy to the configmap and updates the enforcer
func (s *Server) AddPolicy(ctx context.Context, policyKey string, policies string) (*application.RBACPolicyList, error) {
	cm, err := s.fetchConfigMap(ctx)
	if err != nil {
		if !apierr.IsNotFound(err) {
			return nil, err
		}
	} else {
		cm.Data[policyKey] = policies
		err = s.enf.SyncUpdate(cm, func(cm *apiv1.ConfigMap) error { return nil })
		if err != nil {
			return nil, err
		}
	}

	policy := []application.RBACPolicy{{PolicyKey: policyKey, Policy: policies}}
	return &application.RBACPolicyList{Items: policy}, nil
}

// RemovePolicy removes a policy from the configmap and updates the enforcer
func (s *Server) RemovePolicy(ctx context.Context, policyKey string) (*application.RBACPolicyList, error) {
	cm, err := s.fetchConfigMap(ctx)
	if err != nil {
		if !apierr.IsNotFound(err) {
			return nil, err
		}
	} else {
		var policy []application.RBACPolicy
		if val, ok := cm.Data[policyKey]; ok {
			policy = append(policy, application.RBACPolicy{
				PolicyKey: policyKey,
				Policy:    val,
			})
		} else {
			return nil, fmt.Errorf("policyKey %s not found in configmap", policyKey)
		}
		delete(cm.Data, policyKey)
		err = s.enf.SyncUpdate(cm, func(cm *apiv1.ConfigMap) error { return nil })
		if err != nil {
			return nil, err
		}
		return &application.RBACPolicyList{Items: policy}, nil
	}
	return nil, nil
}

// ListPolicies returns all policies
func (s *Server) ListPolicies(ctx context.Context) (*application.RBACPolicyList, error) {
	cm, err := s.fetchConfigMap(ctx)
	if err != nil {
		if !apierr.IsNotFound(err) {
			return nil, err
		}
	} else {
		var policy []application.RBACPolicy
		for k, v := range cm.Data {
			policy = append(policy, application.RBACPolicy{
				PolicyKey: k,
				Policy:    v,
			})
		}
		return &application.RBACPolicyList{Items: policy}, nil
	}
	return nil, nil
}

// GetPolicy returns a policy for a given key
func (s *Server) GetPolicy(ctx context.Context, policyKey string) (*application.RBACPolicy, error) {
	cm, err := s.fetchConfigMap(ctx)
	if err != nil {
		if !apierr.IsNotFound(err) {
			return nil, err
		}
	} else {
		if val, ok := cm.Data[policyKey]; ok {
			return &application.RBACPolicy{
				PolicyKey: policyKey,
				Policy:    val,
			}, nil
		} else {
			return nil, fmt.Errorf("policyKey %s not found in configmap", policyKey)
		}
	}
	return nil, nil
}
