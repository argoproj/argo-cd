package db

import (
	"fmt"
	"hash/fnv"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	informerv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v2/common"
)

func (db *db) listSecretsByType(types ...string) ([]*apiv1.Secret, error) {
	labelSelector := labels.NewSelector()
	req, err := labels.NewRequirement(common.LabelKeySecretType, selection.Equals, types)
	if err != nil {
		return nil, err
	}
	labelSelector = labelSelector.Add(*req)

	secretsLister, err := db.settingsMgr.GetSecretsLister()
	if err != nil {
		return nil, err
	}
	secrets, err := secretsLister.Secrets(db.ns).List(labelSelector)
	if err != nil {
		return nil, err
	}
	return secrets, nil
}

//nolint:unused
func boolOrDefault(secret *apiv1.Secret, key string, def bool) (bool, error) {
	val, present := secret.Data[key]
	if !present {
		return def, nil
	}

	return strconv.ParseBool(string(val))
}

//nolint:unused
func intOrDefault(secret *apiv1.Secret, key string, def int64) (int64, error) {
	val, present := secret.Data[key]
	if !present {
		return def, nil
	}

	return strconv.ParseInt(string(val), 10, 64)
}

func (db *db) createSecret(ctx context.Context, secretType string, secret *apiv1.Secret) (*apiv1.Secret, error) {
	if secret.Annotations == nil {
		secret.Annotations = map[string]string{}
	}
	secret.Annotations[common.AnnotationKeyManagedBy] = common.AnnotationValueManagedByArgoCD

	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}
	secret.Labels[common.LabelKeySecretType] = secretType

	secret, err := db.kubeclientset.CoreV1().Secrets(db.ns).Create(ctx, secret, metav1.CreateOptions{})
	return secret, err
}

func (db *db) deleteSecret(ctx context.Context, secret *apiv1.Secret) error {
	var err error

	canDelete := secret.Annotations != nil && secret.Annotations[common.AnnotationKeyManagedBy] == common.AnnotationValueManagedByArgoCD
	if canDelete {
		err = db.kubeclientset.CoreV1().Secrets(db.ns).Delete(ctx, secret.Name, metav1.DeleteOptions{})
	} else {
		delete(secret.Labels, common.LabelKeySecretType)
		_, err = db.kubeclientset.CoreV1().Secrets(db.ns).Update(ctx, secret, metav1.UpdateOptions{})
	}

	return err
}

func (db *db) watchSecrets(ctx context.Context,
	secretType string,
	handleAddEvent func(secret *apiv1.Secret),
	handleModEvent func(oldSecret *apiv1.Secret, newSecret *apiv1.Secret),
	handleDeleteEvent func(secret *apiv1.Secret)) {

	secretListOptions := func(options *metav1.ListOptions) {
		labelSelector := fields.ParseSelectorOrDie(common.LabelKeySecretType + "=" + secretType)
		options.LabelSelector = labelSelector.String()
	}
	secretEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if secretObj, ok := obj.(*apiv1.Secret); ok {
				handleAddEvent(secretObj)
			}
		},
		DeleteFunc: func(obj interface{}) {
			if secretObj, ok := obj.(*apiv1.Secret); ok {
				handleDeleteEvent(secretObj)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if oldSecretObj, ok := oldObj.(*apiv1.Secret); ok {
				if newSecretObj, ok := newObj.(*apiv1.Secret); ok {
					handleModEvent(oldSecretObj, newSecretObj)
				}
			}
		},
	}

	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
	clusterSecretInformer := informerv1.NewFilteredSecretInformer(db.kubeclientset, db.ns, 3*time.Minute, indexers, secretListOptions)
	clusterSecretInformer.AddEventHandler(secretEventHandler)

	log.Info("Starting secretInformer for", secretType)
	go func() {
		clusterSecretInformer.Run(ctx.Done())
		log.Info("secretInformer for", secretType, "cancelled")
	}()
	<-ctx.Done()
}

// uriToSecretName hashes an uri address to the secret name using a formula.
// Part of the uri address is incorporated for debugging purposes
func URIToSecretName(uriType, uri string) (string, error) {
	parsedURI, err := url.ParseRequestURI(uri)
	if err != nil {
		return "", err
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(uri))
	host := strings.ToLower(strings.Split(parsedURI.Host, ":")[0])
	return fmt.Sprintf("%s-%s-%v", uriType, host, h.Sum32()), nil
}
