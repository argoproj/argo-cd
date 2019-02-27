package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/diff"
)

type ResourcesBatch struct {
	GVK                 schema.GroupVersionKind
	ResourceInfo        metav1.APIResource
	ListResourceVersion string
	Objects             []unstructured.Unstructured
	Error               error
}

type CacheRefreshEvent struct {
	GVK             schema.GroupVersionKind
	ResourceVersion string
	Objects         []unstructured.Unstructured
}

type WatchEvent struct {
	WatchEvent   *watch.Event
	CacheRefresh *CacheRefreshEvent
}

type CachedVersionSource func(gk schema.GroupKind) (string, error)

type Kubectl interface {
	ApplyResource(config *rest.Config, obj *unstructured.Unstructured, namespace string, dryRun, force bool) (string, error)
	ConvertToVersion(obj *unstructured.Unstructured, group, version string) (*unstructured.Unstructured, error)
	DeleteResource(config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, forceDelete bool) error
	GetResource(config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) (*unstructured.Unstructured, error)
	PatchResource(config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, patchType types.PatchType, patchBytes []byte) (*unstructured.Unstructured, error)
	WatchResources(ctx context.Context, config *rest.Config, resourceFilter ResourceFilter, versionSource CachedVersionSource) (chan WatchEvent, error)
	GetResources(config *rest.Config, resourceFilter ResourceFilter, namespace string) (chan ResourcesBatch, error)
	GetAPIResources(config *rest.Config) ([]*metav1.APIResourceList, error)
}

type KubectlCmd struct{}

func (k KubectlCmd) GetAPIResources(config *rest.Config) ([]*metav1.APIResourceList, error) {
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	return disco.ServerResources()
}

// GetResources returns all kubernetes resources
func (k KubectlCmd) GetResources(config *rest.Config, resourceFilter ResourceFilter, namespace string) (chan ResourcesBatch, error) {
	res := make(chan ResourcesBatch)

	listSupported := func(apiResource *metav1.APIResource) bool {
		return isSupportedVerb(apiResource, listVerb)
	}
	apiResIfs, err := filterAPIResources(config, resourceFilter, listSupported, namespace)
	if err != nil {
		return nil, err
	}

	go func() {
		defer close(res)
		var wg sync.WaitGroup
		wg.Add(len(apiResIfs))
		for _, apiResIf := range apiResIfs {
			go func(apiResIf apiResourceInterface) {
				defer wg.Done()
				batch := ResourcesBatch{
					GVK:          apiResIf.groupVersion.WithKind(apiResIf.apiResource.Kind),
					ResourceInfo: apiResIf.apiResource,
				}
				list, err := apiResIf.resourceIf.List(metav1.ListOptions{})
				if err != nil {
					if !apierr.IsNotFound(err) {
						batch.Error = err
						res <- batch
					}
					return
				}
				batch.ListResourceVersion = list.GetResourceVersion()
				batch.Objects = list.Items
				res <- batch
			}(apiResIf)
		}
		wg.Wait()
	}()

	return res, nil
}

const watchResourcesRetryTimeout = 1 * time.Second

// WatchResources watches all the existing resources in the cluster provided by the config. Method retries watch with the most recent resource version stored in cache.
// The WatchResources returns channel which container either watch event with updated resource info or new list of resources if cached resource version had expired.
func (k KubectlCmd) WatchResources(
	ctx context.Context,
	config *rest.Config,
	resourceFilter ResourceFilter,
	versionSource CachedVersionSource,
) (chan WatchEvent, error) {
	watchSupported := func(apiResource *metav1.APIResource) bool {
		return isSupportedVerb(apiResource, watchVerb)
	}
	log.Infof("Start watching for resources changes with in cluster %s", config.Host)
	apiResIfs, err := filterAPIResources(config, resourceFilter, watchSupported, "")
	if err != nil {
		return nil, err
	}
	ch := make(chan WatchEvent)
	go func() {
		var wg sync.WaitGroup
		wg.Add(len(apiResIfs))
		for _, a := range apiResIfs {
			go func(apiResIf apiResourceInterface) {
				defer wg.Done()

				util.RetryUntilSucceed(func() (err error) {
					defer func() {
						if r := recover(); r != nil {
							message := fmt.Sprintf("Recovered from panic: %+v\n%s", r, debug.Stack())
							log.Error(message)
							err = errors.New(message)
						}
					}()
					watchCh := WatchWithRetry(ctx, func() (i watch.Interface, e error) {
						gvk := apiResIf.groupVersion.WithKind(apiResIf.apiResource.Kind)
						resourceVersion, err := versionSource(gvk.GroupKind())
						if err != nil {
							return nil, err
						}
						w, err := apiResIf.resourceIf.Watch(metav1.ListOptions{
							ResourceVersion: resourceVersion,
						})
						if apierr.IsGone(err) {
							log.Infof("Resource version of %s has expired at cluster %s, reloading list", gvk, config.Host)
							list, err := apiResIf.resourceIf.List(metav1.ListOptions{})
							if err != nil {
								return nil, err
							}
							ch <- WatchEvent{
								CacheRefresh: &CacheRefreshEvent{
									GVK:             gvk,
									ResourceVersion: list.GetResourceVersion(),
									Objects:         list.Items,
								},
							}
							return apiResIf.resourceIf.Watch(metav1.ListOptions{ResourceVersion: list.GetResourceVersion()})
						}
						return w, err
					})
					for next := range watchCh {
						if next.Error != nil {
							return next.Error
						}
						ch <- WatchEvent{
							WatchEvent: &watch.Event{
								Object: next.Object,
								Type:   next.Type,
							},
						}
					}
					return nil
				}, fmt.Sprintf("watch resources %s %s/%s", config.Host, apiResIf.groupVersion, apiResIf.apiResource.Kind), ctx, watchResourcesRetryTimeout)
			}(a)
		}
		wg.Wait()
		close(ch)
		log.Infof("Stop watching for resources changes with in cluster %s", config.Host)
	}()
	return ch, nil
}

// GetResource returns resource
func (k KubectlCmd) GetResource(config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) (*unstructured.Unstructured, error) {
	dynamicIf, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	apiResource, err := ServerResourceForGroupVersionKind(disco, gvk)
	if err != nil {
		return nil, err
	}
	resource := gvk.GroupVersion().WithResource(apiResource.Name)
	resourceIf := ToResourceInterface(dynamicIf, apiResource, resource, namespace)
	return resourceIf.Get(name, metav1.GetOptions{})
}

// PatchResource patches resource
func (k KubectlCmd) PatchResource(config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, patchType types.PatchType, patchBytes []byte) (*unstructured.Unstructured, error) {
	dynamicIf, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	apiResource, err := ServerResourceForGroupVersionKind(disco, gvk)
	if err != nil {
		return nil, err
	}
	resource := gvk.GroupVersion().WithResource(apiResource.Name)
	resourceIf := ToResourceInterface(dynamicIf, apiResource, resource, namespace)
	return resourceIf.Patch(name, patchType, patchBytes, metav1.UpdateOptions{})
}

// DeleteResource deletes resource
func (k KubectlCmd) DeleteResource(config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, forceDelete bool) error {
	dynamicIf, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}
	apiResource, err := ServerResourceForGroupVersionKind(disco, gvk)
	if err != nil {
		return err
	}
	resource := gvk.GroupVersion().WithResource(apiResource.Name)
	resourceIf := ToResourceInterface(dynamicIf, apiResource, resource, namespace)
	propagationPolicy := metav1.DeletePropagationForeground
	deleteOptions := &metav1.DeleteOptions{PropagationPolicy: &propagationPolicy}
	if forceDelete {
		propagationPolicy = metav1.DeletePropagationBackground
		zeroGracePeriod := int64(0)
		deleteOptions.GracePeriodSeconds = &zeroGracePeriod
	}

	return resourceIf.Delete(name, deleteOptions)
}

// ApplyResource performs an apply of a unstructured resource
func (k KubectlCmd) ApplyResource(config *rest.Config, obj *unstructured.Unstructured, namespace string, dryRun, force bool) (string, error) {
	log.Infof("Applying resource %s/%s in cluster: %s, namespace: %s", obj.GetKind(), obj.GetName(), config.Host, namespace)
	f, err := ioutil.TempFile(util.TempDir, "")
	if err != nil {
		return "", fmt.Errorf("Failed to generate temp file for kubeconfig: %v", err)
	}
	_ = f.Close()
	err = WriteKubeConfig(config, namespace, f.Name())
	if err != nil {
		return "", fmt.Errorf("Failed to write kubeconfig: %v", err)
	}
	defer util.DeleteFile(f.Name())
	manifestBytes, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	var out []string
	// If it is an RBAC resource, run `kubectl auth reconcile`. This is preferred over
	// `kubectl apply`, which cannot tolerate changes in roleRef, which is an immutable field.
	// See: https://github.com/kubernetes/kubernetes/issues/66353
	// `auth reconcile` will delete and recreate the resource if necessary
	if obj.GetAPIVersion() == "rbac.authorization.k8s.io/v1" {
		// `kubectl auth reconcile` has a side effect of auto-creating namespaces if it doesn't exist.
		// See: https://github.com/kubernetes/kubernetes/issues/71185. This is behavior which we do
		// not want. We need to check if the namespace exists, before know if it is safe to run this
		// command. Skip this for dryRuns.
		if !dryRun && namespace != "" {
			kubeClient, err := kubernetes.NewForConfig(config)
			if err != nil {
				return "", err
			}
			_, err = kubeClient.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
			if err != nil {
				return "", err
			}
		}
		outReconcile, err := runKubectl(f.Name(), namespace, []string{"auth", "reconcile"}, manifestBytes, dryRun)
		if err != nil {
			return "", err
		}
		out = append(out, outReconcile)
		// We still want to fallthrough and run `kubectl apply` in order set the
		// last-applied-configuration annotation in the object.
	}

	// Run kubectl apply
	applyArgs := []string{"apply"}
	if force {
		applyArgs = append(applyArgs, "--force")
	}
	outApply, err := runKubectl(f.Name(), namespace, applyArgs, manifestBytes, dryRun)
	if err != nil {
		return "", err
	}
	out = append(out, outApply)
	return strings.Join(out, ". "), nil
}

func runKubectl(kubeconfigPath string, namespace string, args []string, manifestBytes []byte, dryRun bool) (string, error) {
	cmdArgs := append([]string{"--kubeconfig", kubeconfigPath, "-f", "-"}, args...)
	if namespace != "" {
		cmdArgs = append(cmdArgs, "-n", namespace)
	}
	if dryRun {
		cmdArgs = append(cmdArgs, "--dry-run")
	}
	cmd := exec.Command("kubectl", cmdArgs...)
	log.Info(cmd.Args)
	if log.IsLevelEnabled(log.DebugLevel) {
		var obj unstructured.Unstructured
		err := json.Unmarshal(manifestBytes, &obj)
		if err != nil {
			return "", err
		}
		redacted, _, err := diff.HideSecretData(&obj, nil)
		if err != nil {
			return "", err
		}
		redactedBytes, err := json.Marshal(redacted)
		if err != nil {
			return "", err
		}
		log.Debug(string(redactedBytes))
	}
	cmd.Stdin = bytes.NewReader(manifestBytes)
	out, err := cmd.Output()
	if err != nil {
		if exErr, ok := err.(*exec.ExitError); ok {
			errMsg := cleanKubectlOutput(string(exErr.Stderr))
			return "", errors.New(errMsg)
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ConvertToVersion converts an unstructured object into the specified group/version
func (k KubectlCmd) ConvertToVersion(obj *unstructured.Unstructured, group, version string) (*unstructured.Unstructured, error) {
	gvk := obj.GroupVersionKind()
	if gvk.Group == group && gvk.Version == version {
		return obj.DeepCopy(), nil
	}
	manifestBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	f, err := ioutil.TempFile(util.TempDir, "")
	if err != nil {
		return nil, fmt.Errorf("Failed to generate temp file for kubectl: %v", err)
	}
	_ = f.Close()
	if err := ioutil.WriteFile(f.Name(), manifestBytes, 0600); err != nil {
		return nil, err
	}
	defer util.DeleteFile(f.Name())
	outputVersion := fmt.Sprintf("%s/%s", group, version)
	cmd := exec.Command("kubectl", "convert", "--output-version", outputVersion, "-o", "json", "--local=true", "-f", f.Name())
	cmd.Stdin = bytes.NewReader(manifestBytes)
	out, err := cmd.Output()
	if err != nil {
		if exErr, ok := err.(*exec.ExitError); ok {
			errMsg := cleanKubectlOutput(string(exErr.Stderr))
			return nil, errors.New(errMsg)
		}
		return nil, fmt.Errorf("failed to convert %s/%s to %s/%s", obj.GetKind(), obj.GetName(), group, version)
	}
	// NOTE: when kubectl convert runs against stdin (i.e. kubectl convert -f -), the output is
	// a unstructured list instead of an unstructured object
	var convertedObj unstructured.Unstructured
	err = json.Unmarshal(out, &convertedObj)
	if err != nil {
		return nil, err
	}
	if convertedObj.GetNamespace() == "" {
		convertedObj.SetNamespace(obj.GetNamespace())
	}
	return &convertedObj, nil
}
