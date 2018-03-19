
import "github.com/argoproj/argo-cd/common"

// CreateSecret stores a new secret in Kubernetes.
func (s *Server) CreateSecret(name, value, secretType string) (secret *apiv1.Secret, err error) {
	newSecret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secName,
			Labels: map[string]string{
				common.LabelKeySecretType: secretType,
			},
		},
		StringData: value,
	}
	secret, err = s.kubeclientset.CoreV1().Secrets(s.ns).Create(newSecret)
	if err != nil {
		secret = nil
	}
	return
}

// ReadSecret retrieves a secret from Kubernetes.
func (s *Server) ReadSecret(name string) (secret *apiv1.Secret, err error) {
	secret, err = s.kubeclientset.CoreV1().Secrets(s.ns).Get(name, metav1.GetOptions{})
	if err != nil {
		secret = nil
	}
	return
}

// UpdateSecret updates an existing secret in Kubernetes.
func (s *Server) UpdateSecret(name, value string) (secret *apiv1.Secret, err error) {
	existingSecret = s.ReadSecret(name)
	existingSecret.StringData = value
	secret, err = s.kubeclientset.CoreV1().Secrets(s.ns).Update(existingSecret)
	if err != nil {
		secret = nil
	}
	return
}

// DeleteSecret removes a secret from Kubernetes.
func (s *Server) DeleteSecret(name string) (err error) {
	err = s.kubeclientset.CoreV1().Secrets(s.ns).Delete(name, &metav1.DeleteOptions{})
	return
}
