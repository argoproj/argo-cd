package controller

import (
	"context"
	"encoding/json"
	"runtime/debug"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/git"
)

type SecretController struct {
	kubeClient     kubernetes.Interface
	secretQueue    workqueue.RateLimitingInterface
	secretInformer cache.SharedIndexInformer
	repoClientset  reposerver.Clientset
	namespace      string
}

func (ctrl *SecretController) Run(ctx context.Context) {
	go ctrl.secretInformer.Run(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), ctrl.secretInformer.HasSynced) {
		log.Error("Timed out waiting for caches to sync")
		return
	}
	go wait.Until(func() {
		for ctrl.processSecret() {
		}
	}, time.Second, ctx.Done())
}

func (ctrl *SecretController) processSecret() (processNext bool) {
	secretKey, shutdown := ctrl.secretQueue.Get()
	if shutdown {
		processNext = false
		return
	} else {
		processNext = true
	}

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Recovered from panic: %+v\n%s", r, debug.Stack())
		}
		ctrl.secretQueue.Done(secretKey)
	}()
	obj, exists, err := ctrl.secretInformer.GetIndexer().GetByKey(secretKey.(string))
	if err != nil {
		log.Errorf("Failed to get secret '%s' from informer index: %+v", secretKey, err)
		return
	}
	if !exists {
		// This happens after secret was deleted, but the work queue still had an entry for it.
		return
	}
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		log.Warnf("Key '%s' in index is not an secret", secretKey)
		return
	}

	if secret.Labels[common.LabelKeySecretType] == common.SecretTypeCluster {
		cluster := db.SecretToCluster(secret)
		ctrl.updateState(secret, ctrl.getClusterState(cluster))
	} else if secret.Labels[common.LabelKeySecretType] == common.SecretTypeRepository {
		repo := db.SecretToRepo(secret)
		ctrl.updateState(secret, ctrl.getRepoConnectionState(repo))
	}

	return
}

func (ctrl *SecretController) getRepoConnectionState(repo *v1alpha1.Repository) v1alpha1.ConnectionState {
	state := v1alpha1.ConnectionState{
		ModifiedAt: repo.ConnectionState.ModifiedAt,
		Status:     v1alpha1.ConnectionStatusUnknown,
	}
	err := git.TestRepo(repo.Repo, repo.Username, repo.Password, repo.SSHPrivateKey)
	if err == nil {
		state.Status = v1alpha1.ConnectionStatusSuccessful
	} else {
		state.Status = v1alpha1.ConnectionStatusFailed
		state.Message = err.Error()
	}
	return state
}

func (ctrl *SecretController) getClusterState(cluster *v1alpha1.Cluster) v1alpha1.ConnectionState {
	state := v1alpha1.ConnectionState{
		ModifiedAt: cluster.ConnectionState.ModifiedAt,
		Status:     v1alpha1.ConnectionStatusUnknown,
	}
	kubeClientset, err := kubernetes.NewForConfig(cluster.RESTConfig())
	if err == nil {
		_, err = kubeClientset.Discovery().ServerVersion()
	}
	if err == nil {
		state.Status = v1alpha1.ConnectionStatusSuccessful
	} else {
		state.Status = v1alpha1.ConnectionStatusFailed
		state.Message = err.Error()
	}

	return state
}

func (ctrl *SecretController) updateState(secret *corev1.Secret, state v1alpha1.ConnectionState) {
	annotationsPatch := make(map[string]string)
	for key, value := range db.AnnotationsFromConnectionState(&state) {
		if secret.Annotations[key] != value {
			annotationsPatch[key] = value
		}
	}
	if len(annotationsPatch) > 0 {
		annotationsPatch[common.AnnotationConnectionModifiedAt] = metav1.Now().Format(time.RFC3339)
		patchData, err := json.Marshal(map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": annotationsPatch,
			},
		})
		if err != nil {
			log.Warnf("Unable to prepare secret state annotation patch: %v", err)
		} else {
			_, err := ctrl.kubeClient.CoreV1().Secrets(secret.Namespace).Patch(secret.Name, types.MergePatchType, patchData)
			if err != nil {
				log.Warnf("Unable to patch secret state annotation: %v", err)
			}
		}
	}
}

func newSecretInformer(client kubernetes.Interface, resyncPeriod time.Duration, namespace string, secretQueue workqueue.RateLimitingInterface) cache.SharedIndexInformer {
	informerFactory := informers.NewFilteredSharedInformerFactory(
		client,
		resyncPeriod,
		namespace,
		func(options *metav1.ListOptions) {
			var req *labels.Requirement
			req, err := labels.NewRequirement(common.LabelKeySecretType, selection.In, []string{common.SecretTypeCluster, common.SecretTypeRepository})
			if err != nil {
				panic(err)
			}

			options.FieldSelector = fields.Everything().String()
			labelSelector := labels.NewSelector().Add(*req)
			options.LabelSelector = labelSelector.String()
		},
	)
	informer := informerFactory.Core().V1().Secrets().Informer()
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				key, err := cache.MetaNamespaceKeyFunc(obj)
				if err == nil {
					secretQueue.Add(key)
				}
			},
			UpdateFunc: func(old, new interface{}) {
				key, err := cache.MetaNamespaceKeyFunc(new)
				if err == nil {
					secretQueue.Add(key)
				}
			},
		},
	)
	return informer
}

func NewSecretController(kubeClient kubernetes.Interface, repoClientset reposerver.Clientset, resyncPeriod time.Duration, namespace string) *SecretController {
	secretQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	return &SecretController{
		kubeClient:     kubeClient,
		secretQueue:    secretQueue,
		secretInformer: newSecretInformer(kubeClient, resyncPeriod, namespace, secretQueue),
		namespace:      namespace,
		repoClientset:  repoClientset,
	}
}
