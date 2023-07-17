package kube

import (
	"context"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type kubeUtil struct {
	client      kubernetes.Interface
	ctx         context.Context
	labels      map[string]string
	annotations map[string]string
}

// updateFn will be called to set data for secret s. new will be true if the
// secret was created by the caller, or false if it has existed before.
type updateFn func(s *apiv1.Secret, new bool) error

// NewUtil returns a new kubeUtil receiver
func NewKubeUtil(client kubernetes.Interface, ctx context.Context) *kubeUtil {
	return &kubeUtil{client: client, ctx: ctx}
}

// CreateOrUpdateSecret creates or updates a secret, using the update function.
// If the secret is created, its labels and annotations are set if non-empty in
// the receiver. If the secret is updated, labels and annotations will not be
// touched.
func (ku *kubeUtil) CreateOrUpdateSecret(ns string, name string, update updateFn) error {
	var s *apiv1.Secret
	var err error
	var new bool

	s, err = ku.client.CoreV1().Secrets(ns).Get(ku.ctx, name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		new = true
	}

	if new {
		s = &apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Namespace:   ns,
				Labels:      ku.labels,
				Annotations: ku.annotations,
			},
		}
		s.Data = make(map[string][]byte)
	}

	err = update(s, new)
	if err != nil {
		return err
	}

	if new {
		_, err = ku.client.CoreV1().Secrets(ns).Create(ku.ctx, s, metav1.CreateOptions{})
	} else {
		_, err = ku.client.CoreV1().Secrets(ns).Update(ku.ctx, s, metav1.UpdateOptions{})
	}

	return err

}

// CreateOrUpdateSecretField creates or updates a secret name in namespace ns, with given value for given field
func (ku *kubeUtil) CreateOrUpdateSecretField(ns string, name string, field string, value string) error {
	err := ku.CreateOrUpdateSecret(ns, name, func(s *apiv1.Secret, new bool) error {
		s.Data[field] = []byte(value)
		return nil
	})
	return err
}

// CreateOrUpdateSecretData creates or updates a secret name in namespace ns, with given data.
// If merge is true, merges data with the existing data, otherwise overwrites it.
func (ku *kubeUtil) CreateOrUpdateSecretData(ns string, name string, data map[string][]byte, merge bool) error {
	err := ku.CreateOrUpdateSecret(ns, name, func(s *apiv1.Secret, new bool) error {
		if !merge || new {
			s.Data = data
		} else {
			for key, val := range data {
				s.Data[key] = val
			}
		}
		return nil
	})
	return err
}

// DeepCopy returns a copy of ku
func (ku *kubeUtil) DeepCopy() *kubeUtil {
	kun := &kubeUtil{
		client:      ku.client,
		ctx:         ku.ctx,
		labels:      ku.labels,
		annotations: ku.annotations,
	}
	return kun
}

// WithLabels returns a copy of ku with labels attached
func (ku *kubeUtil) WithLabels(labels map[string]string) *kubeUtil {
	kun := ku.DeepCopy()
	kun.labels = labels
	return kun
}

// WithAnnotations returns a copy of ku with annotations attached
func (ku *kubeUtil) WithAnnotations(annotations map[string]string) *kubeUtil {
	kun := ku.DeepCopy()
	kun.annotations = annotations
	return kun
}
