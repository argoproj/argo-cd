package kube

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/podutils"

	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

func selectPodForPortForward(clientSet kubernetes.Interface, namespace string, podSelectors ...string) (*corev1.Pod, error) {
	for _, podSelector := range podSelectors {
		pods, err := clientSet.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: podSelector,
		})
		if err != nil {
			return nil, err
		}

		for _, po := range pods.Items {
			if po.Status.Phase == corev1.PodRunning && podutils.IsPodReady(&po) {
				return &po, nil
			}
		}
	}
	return nil, fmt.Errorf("cannot find ready pod with selector: %v - use the --{component}-name flag in this command or set the environmental variable (Refer to https://argo-cd.readthedocs.io/en/stable/user-guide/environment-variables), to change the Argo CD component name in the CLI", podSelectors)
}

func PortForward(targetPort int, namespace string, overrides *clientcmd.ConfigOverrides, podSelectors ...string) (int, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	clientConfig := clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, overrides, os.Stdin)
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return -1, err
	}

	if namespace == "" {
		namespace, _, err = clientConfig.Namespace()
		if err != nil {
			return -1, err
		}
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return -1, err
	}

	pod, err := selectPodForPortForward(clientSet, namespace, podSelectors...)
	if err != nil {
		return -1, err
	}

	url := clientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(pod.Namespace).
		Name(pod.Name).
		SubResource("portforward").URL()

	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return -1, fmt.Errorf("could not create round tripper: %w", err)
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", url)

	// Reuse environment variable for kubectl to disable the feature flag, default is enabled.
	if !cmdutil.PortForwardWebsockets.IsDisabled() {
		tunnelingDialer, err := portforward.NewSPDYOverWebsocketDialer(url, config)
		if err != nil {
			return -1, fmt.Errorf("could not create tunneling dialer: %w", err)
		}
		// First attempt tunneling (websocket) dialer, then fallback to spdy dialer.
		dialer = portforward.NewFallbackDialer(tunnelingDialer, dialer, func(err error) bool {
			return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
		})
	}

	readyChan := make(chan struct{}, 1)
	failedChan := make(chan error, 1)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)

	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return -1, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	utilio.Close(ln)
	forwarder, err := portforward.NewOnAddresses(dialer, []string{"localhost"}, []string{fmt.Sprintf("%d:%d", port, targetPort)}, context.Background().Done(), readyChan, out, errOut)
	if err != nil {
		return -1, err
	}

	go func() {
		err = forwarder.ForwardPorts()
		if err != nil {
			failedChan <- err
		}
	}()
	select {
	case err = <-failedChan:
		return -1, err
	case <-readyChan:
	}
	if errOut.String() != "" {
		return -1, fmt.Errorf("%s", errOut.String())
	}
	return port, nil
}
