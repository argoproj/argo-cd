package kube

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"github.com/argoproj/argo-cd/v2/util/io"
)

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

	var pod *corev1.Pod

	for _, podSelector := range podSelectors {
		pods, err := clientSet.CoreV1().Pods(namespace).List(context.Background(), v1.ListOptions{
			LabelSelector: podSelector,
		})
		if err != nil {
			return -1, err
		}

		if len(pods.Items) > 0 {
			pod = &pods.Items[0]
			break
		}
	}

	if pod == nil {
		return -1, fmt.Errorf("cannot find pod with selector: %v - use the --{component}-name flag in this command or set the environmental variable (Refer to https://argo-cd.readthedocs.io/en/stable/user-guide/environment-variables), to change the Argo CD component name in the CLI", podSelectors)
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

	readyChan := make(chan struct{}, 1)
	failedChan := make(chan error, 1)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)

	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return -1, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	io.Close(ln)

	forwarder, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", port, targetPort)}, context.Background().Done(), readyChan, out, errOut)
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
	if len(errOut.String()) != 0 {
		return -1, fmt.Errorf("%s", errOut.String())
	}
	return port, nil
}
