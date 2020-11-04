package kube

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"github.com/argoproj/argo-cd/util/io"
)

func PortForward(podSelector string, targetPort int, namespace string) (int, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := clientcmd.ConfigOverrides{}
	clientConfig := clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)
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

	pods, err := clientSet.CoreV1().Pods(namespace).List(context.Background(), v1.ListOptions{
		LabelSelector: podSelector,
	})
	if err != nil {
		return -1, err
	}

	if len(pods.Items) == 0 {
		return -1, fmt.Errorf("cannot find %s pod", podSelector)
	}

	url := clientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(pods.Items[0].Namespace).
		Name(pods.Items[0].Name).
		SubResource("portforward").URL()

	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return -1, errors.Wrap(err, "Could not create round tripper")
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", url)

	readyChan := make(chan struct{}, 1)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)

	ln, err := net.Listen("tcp", "[::]:0")
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
			log.Fatal(err)
		}
	}()
	for range readyChan {
	}
	if len(errOut.String()) != 0 {
		return -1, fmt.Errorf(errOut.String())
	}
	return port, nil
}
