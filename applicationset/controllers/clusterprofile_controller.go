package controllers

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterinventory "sigs.k8s.io/cluster-inventory-api/apis/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	clusterProfileFinalizer = "argoproj.io/cluster-profile-finalizer"
	secretNameTemplate      = "cluster-%s"
	secretConfigTemplate    = `{
	"execProviderConfig": {
		"command": "argocd-k8s-auth",
		"args": ["%s"],
		"apiVersion": "client.authentication.k8s.io/v1beta1"
	},
	"tlsClientConfig": {
		"insecure": false,
		"caData": "%s"
	}
}`
)

// ClusterProfileReconciler reconciles a ClusterProfile object with a corresponding Secret
type ClusterProfileReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Namespace string
}

//+kubebuilder:rbac:groups=multicluster.x-k8s.io,resources=clusterprofiles,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

func (r *ClusterProfileReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("clusterprofile", req.NamespacedName)

	var clusterProfile clusterinventory.ClusterProfile
	if err := r.Get(ctx, req.NamespacedName, &clusterProfile); err != nil {
		if errors.IsNotFound(err) {
			// The ClusterProfile was deleted
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch ClusterProfile")
		return ctrl.Result{}, err
	}

	// Prune secret when the Cluster Profile is deleted
	if !clusterProfile.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.pruneSecret(ctx, &clusterProfile)
	}

	if !hasArgoCDProvider(&clusterProfile) {
		log.Info("no provider with prefix argocd-config- found, skipping reconciliation")
		return ctrl.Result{}, r.pruneSecret(ctx, &clusterProfile)
	}

	secretName := fmt.Sprintf(secretNameTemplate, clusterProfile.Name)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: r.Namespace,
		},
	}

	// Add finalizer
	if !controllerutil.ContainsFinalizer(&clusterProfile, clusterProfileFinalizer) {
		controllerutil.AddFinalizer(&clusterProfile, clusterProfileFinalizer)
		if err := r.Update(ctx, &clusterProfile); err != nil {
			log.Error(err, "unable to update ClusterProfile")
			return ctrl.Result{}, err
		}
	}

	// Generate and create/update the secret
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, secret, func() error {
		return r.mutateSecret(secret, &clusterProfile)
	})
	if err != nil {
		log.Error(err, "unable to create or update secret for ClusterProfile")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func hasArgoCDProvider(clusterProfile *clusterinventory.ClusterProfile) bool {
	for _, p := range clusterProfile.Status.AccessProviders {
		if strings.HasPrefix(p.Name, "argocd-config-") {
			return true
		}
	}
	return false
}

func (r *ClusterProfileReconciler) pruneSecret(ctx context.Context, clusterProfile *clusterinventory.ClusterProfile) error {
	log := r.Log.WithValues("clusterprofile", clusterProfile.Name)

	// Deletion logic was already handled
	if !controllerutil.ContainsFinalizer(clusterProfile, clusterProfileFinalizer) {
		return nil
	}

	secretName := fmt.Sprintf(secretNameTemplate, clusterProfile.Name)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: r.Namespace,
		},
	}
	if err := r.Delete(ctx, secret); err != nil && !errors.IsNotFound(err) {
		log.Error(err, "unable to delete secret")
		return err
	}

	controllerutil.RemoveFinalizer(clusterProfile, clusterProfileFinalizer)
	if err := r.Update(ctx, clusterProfile); err != nil {
		log.Error(err, "unable to update ClusterProfile for deletion")
		return err
	}
	return nil
}

func (r *ClusterProfileReconciler) mutateSecret(secret *corev1.Secret, clusterProfile *clusterinventory.ClusterProfile) error {
	secret.Labels = map[string]string{
		"argocd.argoproj.io/secret-type": "cluster",
	}
	secret.Annotations = map[string]string{
		"clusterprofile.x-k8s.io/origin": fmt.Sprintf("%s/%s", clusterProfile.Namespace, clusterProfile.Name),
	}

	if len(clusterProfile.Status.AccessProviders) == 0 {
		return fmt.Errorf("ClusterProvider %v missing field Status.AccessProviders", clusterProfile.Name)
	}

	var provider *clusterinventory.AccessProvider
	for i := range clusterProfile.Status.AccessProviders {
		if strings.HasPrefix(clusterProfile.Status.AccessProviders[i].Name, "argocd-config-") {
			provider = &clusterProfile.Status.AccessProviders[i]
			break
		}
	}

	if provider == nil {
		return fmt.Errorf("no provider with prefix argocd-config- found in ClusterProvider %v", clusterProfile.Name)
	}

	providerName := strings.TrimPrefix(provider.Name, "argocd-config-")
	server := provider.Cluster.Server
	caData := base64.StdEncoding.EncodeToString(provider.Cluster.CertificateAuthorityData)

	secret.StringData = map[string]string{
		"name":   providerName,
		"server": server,
		"config": fmt.Sprintf(secretConfigTemplate, providerName, caData),
	}

	return nil
}

func (r *ClusterProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterinventory.ClusterProfile{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
