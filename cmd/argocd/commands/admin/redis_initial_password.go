package admin

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/errors"
)

func generateRandomPassword() (string, error) {
	const initialPasswordLength = 16
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"
	randBytes := make([]byte, initialPasswordLength)
	for i := 0; i < initialPasswordLength; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}
		randBytes[i] = letters[num.Int64()]
	}
	initialPassword := string(randBytes)
	return initialPassword, nil
}

// NewRedisInitialPasswordCommand defines a new command to ensure Argo CD Redis password secret exists.
func NewRedisInitialPasswordCommand() *cobra.Command {
	var clientConfig clientcmd.ClientConfig
	command := cobra.Command{
		Use:   "redis-initial-password",
		Short: "Ensure the Redis password exists, creating a new one if necessary.",
		Run: func(c *cobra.Command, args []string) {
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)

			// redisInitialCredentials is the kubernetes secret containing
			// the redis password
			redisInitialCredentials := common.RedisInitialCredentials

			// redisInitialCredentialsKey is the key in the redisInitialCredentials
			// secret which maps to the redis password
			redisInitialCredentialsKey := common.RedisInitialCredentialsKey
			fmt.Printf("Checking for initial Redis password in secret %s/%s at key %s. \n", namespace, redisInitialCredentials, redisInitialCredentialsKey)

			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			errors.CheckError(v1alpha1.SetK8SConfigDefaults(config))

			kubeClientset := kubernetes.NewForConfigOrDie(config)

			randomPassword, err := generateRandomPassword()
			errors.CheckError(err)

			data := map[string][]byte{
				redisInitialCredentialsKey: []byte(randomPassword),
			}
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      redisInitialCredentials,
					Namespace: namespace,
				},
				Data: data,
				Type: corev1.SecretTypeOpaque,
			}
			_, err = kubeClientset.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
			if err != nil && !apierr.IsAlreadyExists(err) {
				errors.CheckError(err)
			}

			fmt.Printf("Argo CD Redis secret state confirmed: secret name %s.\n", redisInitialCredentials)
			secret, err = kubeClientset.CoreV1().Secrets(namespace).Get(context.Background(), redisInitialCredentials, v1.GetOptions{})
			errors.CheckError(err)

			if _, ok := secret.Data[redisInitialCredentialsKey]; ok {
				fmt.Println("Password secret is configured properly.")
			} else {
				err := fmt.Errorf("key %s doesn't exist in secret %s. \n", redisInitialCredentialsKey, redisInitialCredentials)
				errors.CheckError(err)
			}
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(&command)

	return &command
}
