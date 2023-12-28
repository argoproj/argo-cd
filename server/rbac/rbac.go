package rbac

import (
	"context"
	"fmt"
	application "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"

	rbacpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/rbac"
	"github.com/argoproj/argo-cd/v2/util/rbac"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	ConfigMapPolicyCSVKey     = "policy.csv"
	ConfigMapPolicyDefaultKey = "policy.default"
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
	return &Server{
		enf:           enf,
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
func (s *Server) AddPolicy(ctx context.Context, q *rbacpkg.RBACPolicyUpdateRequest) (*application.RBACPolicy, error) {
	cm, err := s.fetchConfigMap(ctx)
	if err != nil {
		if !apierr.IsNotFound(err) {
			return nil, err
		}
	} else {
		if q.PolicyKey == ConfigMapPolicyCSVKey ||
			q.PolicyKey == ConfigMapPolicyDefaultKey ||
			(strings.HasPrefix(q.PolicyKey, "policy.") &&
				strings.HasSuffix(q.PolicyKey, ".csv")) {
			cm.Data[q.PolicyKey] = q.Policy.Policy
			// Update the ConfigMap
			updatedConfigMap, err := s.kubeClientset.CoreV1().ConfigMaps(s.namespace).Update(ctx, cm, metav1.UpdateOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to update configmap: %v", err)
			}
			err = s.enf.SyncUpdate(updatedConfigMap, func(cm *apiv1.ConfigMap) error { return nil })
			if err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("policyKey %s is not valid", q.PolicyKey)
		}
	}
	return &application.RBACPolicy{PolicyKey: q.PolicyKey, Policy: q.Policy.Policy}, nil
}

// RemovePolicy removes a policy from the configmap and updates the enforcer
func (s *Server) RemovePolicy(ctx context.Context, q *rbacpkg.RBACPolicyQuery) (*application.RBACPolicy, error) {
	cm, err := s.fetchConfigMap(ctx)
	if err != nil {
		if !apierr.IsNotFound(err) {
			return nil, err
		}
	} else {
		policy := application.RBACPolicy{}
		if val, ok := cm.Data[q.PolicyKey]; ok {
			policy = application.RBACPolicy{
				PolicyKey: q.PolicyKey,
				Policy:    val,
			}
		} else {
			return nil, fmt.Errorf("policyKey %s not found in configmap", q.PolicyKey)
		}
		delete(cm.Data, q.PolicyKey)
		updatedConfigMap, err := s.kubeClientset.CoreV1().ConfigMaps(s.namespace).Update(ctx, cm, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to update configmap: %v", err)
		}
		err = s.enf.SyncUpdate(updatedConfigMap, func(cm *apiv1.ConfigMap) error { return nil })
		if err != nil {
			return nil, err
		}
		return &policy, nil
	}
	return nil, nil
}

// ListPolicies returns all policies
func (s *Server) ListPolicies(ctx context.Context, q *rbacpkg.RBACPolicyListRequest) (*application.RBACPolicyList, error) {
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
func (s *Server) GetPolicy(ctx context.Context, q *rbacpkg.RBACPolicyQuery) (*application.RBACPolicy, error) {
	cm, err := s.fetchConfigMap(ctx)
	if err != nil {
		if !apierr.IsNotFound(err) {
			return nil, err
		}
	} else {
		if val, ok := cm.Data[q.PolicyKey]; ok {
			return &application.RBACPolicy{
				PolicyKey: q.PolicyKey,
				Policy:    val,
			}, nil
		} else {
			return nil, fmt.Errorf("policyKey %s not found in configmap", q.PolicyKey)
		}
	}
	return nil, nil
}
