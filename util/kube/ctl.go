package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
	"sync"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Kubectl interface {
	ApplyResource(config *rest.Config, obj *unstructured.Unstructured, namespace string, dryRun, force bool) (string, error)
	ConvertToVersion(obj *unstructured.Unstructured, group, version string) (*unstructured.Unstructured, error)
	DeleteResource(config *rest.Config, obj *unstructured.Unstructured, namespace string) error
	WatchResources(ctx context.Context, config *rest.Config, namespace string, selector func(kind schema.GroupVersionKind) metav1.ListOptions) (chan watch.Event, error)
}

type KubectlCmd struct{}

// WatchResources Watches all the existing resources with the provided label name in the provided namespace in the cluster provided by the config
func (k KubectlCmd) WatchResources(
	ctx context.Context, config *rest.Config, namespace string, selector func(kind schema.GroupVersionKind) metav1.ListOptions) (chan watch.Event, error) {

	log.Infof("Start watching for resources changes with in cluster %s", config.Host)
	dynClientPool := dynamic.NewDynamicClientPool(config)
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	serverResources, err := GetCachedServerResources(config.Host, disco)
	if err != nil {
		return nil, err
	}

	items := make([]struct {
		resource dynamic.ResourceInterface
		gvk      schema.GroupVersionKind
	}, 0)
	for _, apiResourcesList := range serverResources {
		for i := range apiResourcesList.APIResources {
			apiResource := apiResourcesList.APIResources[i]
			watchSupported := false
			for _, verb := range apiResource.Verbs {
				if verb == watchVerb {
					watchSupported = true
					break
				}
			}
			if watchSupported && !isExcludedResourceGroup(apiResource) {
				dclient, err := dynClientPool.ClientForGroupVersionKind(schema.FromAPIVersionAndKind(apiResourcesList.GroupVersion, apiResource.Kind))
				if err != nil {
					return nil, err
				}
				items = append(items, struct {
					resource dynamic.ResourceInterface
					gvk      schema.GroupVersionKind
				}{resource: dclient.Resource(&apiResource, namespace), gvk: schema.FromAPIVersionAndKind(apiResourcesList.GroupVersion, apiResource.Kind)})
			}
		}
	}
	ch := make(chan watch.Event)
	go func() {
		var wg sync.WaitGroup
		wg.Add(len(items))
		for i := 0; i < len(items); i++ {
			item := items[i]
			go func() {
				defer wg.Done()
				w, err := item.resource.Watch(selector(item.gvk))
				if err == nil {
					defer w.Stop()
					copyEventsChannel(ctx, w.ResultChan(), ch)
				}
			}()
		}
		wg.Wait()
		close(ch)
		log.Infof("Stop watching for resources changes with in cluster %s", config.ServerName)
	}()
	return ch, nil
}

// DeleteResource deletes resource
func (k KubectlCmd) DeleteResource(config *rest.Config, obj *unstructured.Unstructured, namespace string) error {
	dynClientPool := dynamic.NewDynamicClientPool(config)
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}
	gvk := obj.GroupVersionKind()
	dclient, err := dynClientPool.ClientForGroupVersionKind(gvk)
	if err != nil {
		return err
	}
	apiResource, err := ServerResourceForGroupVersionKind(disco, gvk)
	if err != nil {
		return err
	}
	reIf := dclient.Resource(apiResource, namespace)
	propagationPolicy := metav1.DeletePropagationForeground
	return reIf.Delete(obj.GetName(), &metav1.DeleteOptions{PropagationPolicy: &propagationPolicy})
}

// ApplyResource performs an apply of a unstructured resource
func (k KubectlCmd) ApplyResource(config *rest.Config, obj *unstructured.Unstructured, namespace string, dryRun, force bool) (string, error) {
	log.Infof("Applying resource %s/%s in cluster: %s, namespace: %s", obj.GetKind(), obj.GetName(), config.Host, namespace)
	f, err := ioutil.TempFile(kubectlTempDir, "")
	if err != nil {
		return "", fmt.Errorf("Failed to generate temp file for kubeconfig: %v", err)
	}
	_ = f.Close()
	err = WriteKubeConfig(config, namespace, f.Name())
	if err != nil {
		return "", fmt.Errorf("Failed to write kubeconfig: %v", err)
	}
	defer deleteFile(f.Name())
	manifestBytes, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	var out []string
	if obj.GetAPIVersion() == "rbac.authorization.k8s.io/v1" {
		outReconcile, err := runKubectl(f.Name(), namespace, []string{"auth", "reconcile"}, manifestBytes, dryRun)
		if err != nil {
			return "", err
		}
		out = append(out, outReconcile)
	}

	applyArgs := []string{"apply"}
	if force {
		applyArgs = append(applyArgs, "--force")
	}
	outApply, err := runKubectl(f.Name(), namespace, applyArgs, manifestBytes, dryRun)
	if err != nil {
		return "", err
	}
	out = append(out, outApply)
	return strings.Join(out, "\n"), nil
}

func runKubectl(kubeconfigPath string, namespace string, args []string, manifestBytes []byte, dryRun bool) (string, error) {
	cmdArgs := append(append([]string{"--kubeconfig", kubeconfigPath, "-n", namespace}, args...), "-f", "-")
	if dryRun {
		cmdArgs = append(cmdArgs, "--dry-run")
	}
	cmd := exec.Command("kubectl", cmdArgs...)
	log.Info(cmd.Args)
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
	f, err := ioutil.TempFile(kubectlTempDir, "")
	if err != nil {
		return nil, fmt.Errorf("Failed to generate temp file for kubectl: %v", err)
	}
	_ = f.Close()
	if err := ioutil.WriteFile(f.Name(), manifestBytes, 0600); err != nil {
		return nil, err
	}
	defer deleteFile(f.Name())
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
	return &convertedObj, nil
}
