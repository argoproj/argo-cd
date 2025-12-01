package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterinventory "sigs.k8s.io/cluster-inventory-api/apis/v1alpha1"
	"sigs.k8s.io/cluster-inventory-api/pkg/credentials"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

const (
	// clusterProfileFinalizer is the finalizer used by the ClusterProfileReconciler to ensure that
	// the corresponding Secret is deleted when the ClusterProfile is deleted.
	clusterProfileFinalizer = "argoproj.io/cluster-profile-finalizer"
	// secretNameTemplate is the template used to generate the name of the Secret for a ClusterProfile.
	secretNameTemplate = "cluster-%s"
	// clusterProfileOriginLabel is the label used to identify the ClusterProfile that a Secret was created from.
	clusterProfileOriginLabel = "argocd.argoproj.io/cluster-profile-origin"
)

// ClusterProfileReconciler reconciles a ClusterProfile object with a corresponding Secret
type ClusterProfileReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Namespace string
	// ClusterProfileProvidersFile is the path to the file containing the cluster profile providers configuration.
	ClusterProfileProvidersFile string
	// AccessProviders is the set of access providers used to build the kubeconfig for a ClusterProfile.
	AccessProviders *credentials.CredentialsProvider
}

//+kubebuilder:rbac:groups=multicluster.x-k8s.io,resources=clusterprofiles,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

func (r *ClusterProfileReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("clusterprofile", req.NamespacedName)

	// Fetch Cluster Profile
	var clusterProfile clusterinventory.ClusterProfile
	if err := r.Get(ctx, req.NamespacedName, &clusterProfile); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch ClusterProfile")
		return ctrl.Result{}, err
	}

	// If the Cluster Profile is being deleted, prune the corresponding secret and remove the finalizer.
	if !clusterProfile.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.pruneSecret(ctx, &clusterProfile)
	}

	// Add finalizer for pruning secret
	if !controllerutil.ContainsFinalizer(&clusterProfile, clusterProfileFinalizer) {
		controllerutil.AddFinalizer(&clusterProfile, clusterProfileFinalizer)
		if err := r.Update(ctx, &clusterProfile); err != nil {
			log.Error(err, "unable to update ClusterProfile")
			return ctrl.Result{}, err
		}
	}

	// Create or update the secret for the ClusterProfile.
	secretName := fmt.Sprintf(secretNameTemplate, clusterProfile.Name)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: r.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, secret, func() error {
		return r.mutateSecret(secret, &clusterProfile)
	})
	if err != nil {
		log.Error(err, "unable to create or update secret for ClusterProfile")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// pruneSecret handles the deletion of the Secret associated with a ClusterProfile.
func (r *ClusterProfileReconciler) pruneSecret(ctx context.Context, clusterProfile *clusterinventory.ClusterProfile) error {
	log := r.Log.WithValues("clusterprofile", clusterProfile.Name)

	// If the finalizer is not present, the deletion logic has already been handled.
	if !controllerutil.ContainsFinalizer(clusterProfile, clusterProfileFinalizer) {
		return nil
	}

	// Construct the secret name from the ClusterProfile name.
	secretName := fmt.Sprintf(secretNameTemplate, clusterProfile.Name)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: r.Namespace,
		},
	}

	// Attempt to delete the secret.
	err := r.Delete(ctx, secret)
	if err != nil && !errors.IsNotFound(err) {
		log.Error(err, "unable to delete secret")
		return err
	}

	// Once the secret is gone, remove the finalizer from the ClusterProfile.
	controllerutil.RemoveFinalizer(clusterProfile, clusterProfileFinalizer)
	if err := r.Update(ctx, clusterProfile); err != nil {
		log.Error(err, "unable to update ClusterProfile for deletion")
		return err
	}
	return nil
}

// mutateSecret populates the secret with data from the ClusterProfile.
func (r *ClusterProfileReconciler) mutateSecret(secret *corev1.Secret, clusterProfile *clusterinventory.ClusterProfile) error {
	// Set labels on the secret to identify it as a cluster secret and link it to the ClusterProfile.
	secret.Labels = map[string]string{
		common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
		clusterProfileOriginLabel: fmt.Sprintf("%s-%s", clusterProfile.Namespace, clusterProfile.Name),
	}

	if len(clusterProfile.Status.AccessProviders) == 0 {
		return fmt.Errorf("ClusterProfile %v field Status.AccessProviders is empty", clusterProfile.Name)
	}
	// Build the kubeconfig from the ClusterProfile's access providers.
	config, err := r.AccessProviders.BuildConfigFromCP(clusterProfile)
	if err != nil {
		return fmt.Errorf("failed to build config: %w", err)
	}

	apiConfig := v1alpha1.ClusterConfig{
		BearerToken: config.BearerToken,
		TLSClientConfig: v1alpha1.TLSClientConfig{
			Insecure:   config.Insecure,
			ServerName: config.ServerName,
			CertData:   config.CertData,
			KeyData:    config.KeyData,
			CAData:     config.CAData,
		},
		DisableCompression: config.DisableCompression,
	}

	// If there is an exec provider, add it to the config.
	if config.ExecProvider != nil {
		apiConfig.ExecProviderConfig = &v1alpha1.ExecProviderConfig{
			Command:    config.ExecProvider.Command,
			Args:       config.ExecProvider.Args,
			APIVersion: config.ExecProvider.APIVersion,
		}
		if len(config.ExecProvider.Env) > 0 {
			apiConfig.ExecProviderConfig.Env = make(map[string]string)
			for _, env := range config.ExecProvider.Env {
				apiConfig.ExecProviderConfig.Env[env.Name] = env.Value
			}
		}
	}

	configBytes, err := json.Marshal(apiConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	secret.StringData = map[string]string{
		"name":   clusterProfile.Name,
		"server": config.Host,
		"config": string(configBytes),
	}

	return nil
}

func (r *ClusterProfileReconciler) loadClusterProfileProviders() error {
	// TODO: do we need to reload periodically? (unlikely)
	if r.ClusterProfileProvidersFile == "" {
		r.Log.Info("no cluster profile providers file specified, skipping")
		return nil
	}
	providers, err := credentials.NewFromFile(r.ClusterProfileProvidersFile)
	if err != nil {
		return fmt.Errorf("failed to get providers from file: %w", err)
	}
	r.AccessProviders = providers
	return nil
}

func (r *ClusterProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := r.loadClusterProfileProviders(); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterinventory.ClusterProfile{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
