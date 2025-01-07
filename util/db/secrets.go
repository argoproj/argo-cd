package db

import (
	"context"
	"fmt"
	"hash/fnv"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
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

func boolOrFalse(secret *apiv1.Secret, key string) (bool, error) {
	val, present := secret.Data[key]
	if !present {
		return false, nil
	}

	return strconv.ParseBool(string(val))
}

func intOrZero(secret *apiv1.Secret, key string) (int64, error) {
	val, present := secret.Data[key]
	if !present {
		return 0, nil
	}

	return strconv.ParseInt(string(val), 10, 64)
}

func updateSecretBool(secret *apiv1.Secret, key string, value bool) {
	if _, present := secret.Data[key]; present || value {
		secret.Data[key] = []byte(strconv.FormatBool(value))
	}
}

func updateSecretInt(secret *apiv1.Secret, key string, value int64) {
	if _, present := secret.Data[key]; present || value != 0 {
		secret.Data[key] = []byte(strconv.FormatInt(value, 10))
	}
}

func updateSecretString(secret *apiv1.Secret, key, value string) {
	if _, present := secret.Data[key]; present || len(value) > 0 {
		secret.Data[key] = []byte(value)
	}
}

func (db *db) createSecret(ctx context.Context, secret *apiv1.Secret) (*apiv1.Secret, error) {
	return db.kubeclientset.CoreV1().Secrets(db.ns).Create(ctx, secret, metav1.CreateOptions{})
}

func addSecretMetadata(secret *apiv1.Secret, secretType string) {
	if secret.Annotations == nil {
		secret.Annotations = map[string]string{}
	}
	secret.Annotations[common.AnnotationKeyManagedBy] = common.AnnotationValueManagedByArgoCD

	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}
	secret.Labels[common.LabelKeySecretType] = secretType
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
	handleDeleteEvent func(secret *apiv1.Secret),
) {
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
	_, err := clusterSecretInformer.AddEventHandler(secretEventHandler)
	if err != nil {
		log.Error(err)
	}

	log.Info("Starting secretInformer for", secretType)
	go func() {
		clusterSecretInformer.Run(ctx.Done())
		log.Info("secretInformer for", secretType, "cancelled")
	}()
	<-ctx.Done()
}

// URIToSecretName hashes an uri address to the secret name using a formula.
// Part of the uri address is incorporated for debugging purposes
func URIToSecretName(uriType, uri string) (string, error) {
	parsedURI, err := url.ParseRequestURI(uri)
	if err != nil {
		return "", err
	}
	host := parsedURI.Host
	if strings.HasPrefix(host, "[") {
		last := strings.Index(host, "]")
		if last >= 0 {
			addr, err := netip.ParseAddr(host[1:last])
			if err != nil {
				return "", err
			}
			host = strings.ReplaceAll(addr.String(), ":", "-")
		}
	} else {
		last := strings.Index(host, ":")
		if last >= 0 {
			host = host[0:last]
		}
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(uri))
	host = strings.ToLower(host)
	return fmt.Sprintf("%s-%s-%v", uriType, host, h.Sum32()), nil
}
