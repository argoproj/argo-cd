package discovery

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"time"

	log "github.com/sirupsen/logrus"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

func (s *plugins) namespace() string {
	ns := os.Getenv("ARGOCD_SERVICE_PLUGINS_NAMESPACE")

	switch ns {
	case `*`:
		return corev1.NamespaceAll
	case "":
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		namespace, _, err := kubeConfig.Namespace()
		if err != nil {
			log.Warnf("Error getting default namespace %s. Got %s", err, namespace)
		}
		return namespace
	default:
		return ns
	}
}

func (s *plugins) serviceWatcher(c *kubernetes.Clientset) {
	// Short refresh here will refresh names more rapidly, in case the
	factory := informers.NewFilteredSharedInformerFactory(c, 1*time.Minute, s.namespace(),
		func(opts *metav1.ListOptions) {
			opts.LabelSelector = "argocd.argoproj.io/plugin=true"
		})

	informer := factory.Core().V1().Services()
	s.informer = &informer

	_, err := informer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    s.svcAdd,
			UpdateFunc: s.svcUpdate,
			DeleteFunc: s.svcDelete,
		},
	)
	if err != nil {
		log.Errorf("Failed to initialize plugin services watcher, plugins as services will not work: %s", err)
		return
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	informer.Informer().Run(ctx.Done())
}

func (s *plugins) svcAdd(obj interface{}) {
	svc := obj.(*corev1.Service)
	log.Infof("Adding plugin service %s", namespacedName(svc))
	s.serviceMutex.Lock()
	defer s.serviceMutex.Unlock()
	s.addFromService(svc)
}

func (s *plugins) svcDelete(obj interface{}) {
	svc := obj.(*corev1.Service)
	log.Infof("Deleting plugin service %s", namespacedName(svc))
	s.serviceMutex.Lock()
	defer s.serviceMutex.Unlock()
	s.deleteByOwner(svc)
}

func (s *plugins) svcUpdate(_ interface{}, new interface{}) {
	svc := new.(*corev1.Service)
	log.Infof("Updating plugin service %s", namespacedName(svc))
	s.serviceMutex.Lock()
	defer s.serviceMutex.Unlock()
	// Simple delete all and add them all again logic.
	// Optimising this seems premature.
	s.deleteByOwner(svc)
	s.addFromService(svc)
}

func namespacedName(svc *corev1.Service) types.NamespacedName {
	return types.NamespacedName{Name: svc.ObjectMeta.Name, Namespace: svc.ObjectMeta.Namespace}
}

func (s *plugins) deleteByOwner(svc *corev1.Service) {
	namespace := svc.ObjectMeta.Namespace
	name := svc.ObjectMeta.Name
	// You must have the rw lock to call this
	s.servicePlugins = slices.DeleteFunc(s.servicePlugins,
		func(svc *plugin) bool {
			return svc.owner.namespace == namespace && svc.owner.serviceName == name
		})
}

func getNameFromSvc(p *plugin) string {
	_, cmpClient, err := getCmpClient(p)
	if err != nil {
		log.Errorf("Error connecting to cmp service %v", err)
		return ""
	}
	specificationStream, err := cmpClient.GetSpecification(
		context.Background(),
		grpc_retry.Disable(),
	)
	if err != nil {
		log.Errorf("Error getting specification %v", err)
	}
	specificationResp, err := specificationStream.CloseAndRecv()
	if err != nil {
		log.Errorf("Error getting specification response %v", err)
	}
	return specificationResp.GetName()
}

func (s *plugins) addFromService(svc *corev1.Service) {
	// You must have the rw lock to call this
	for _, port := range svc.Spec.Ports {
		address := fmt.Sprintf("%s.%s.svc.cluster.local:%d",
			svc.ObjectMeta.Name, svc.ObjectMeta.Namespace, port.Port)
		p := plugin{
			pluginType: service,
			address:    address,
			owner: pluginOwner{
				namespace:   svc.ObjectMeta.Namespace,
				serviceName: svc.ObjectMeta.Name,
				portName:    port.Name,
			},
		}
		p.name = s.getName(&p)
		s.servicePlugins = append(s.servicePlugins, &p)
	}
}
