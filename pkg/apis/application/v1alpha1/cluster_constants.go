package v1alpha1

import (
	"os"
	"strconv"
)

const (

	// EnvVarFakeInClusterConfig is an environment variable to fake an in-cluster RESTConfig using
	// the current kubectl context (for development purposes)
	EnvVarFakeInClusterConfig = "ARGOCD_FAKE_IN_CLUSTER"

	// EnvK8sClientQPS is the QPS value used for the kubernetes client (default: 50)
	EnvK8sClientQPS = "ARGOCD_K8S_CLIENT_QPS"

	// EnvK8sClientBurst is the burst value used for the kubernetes client (default: twice the client QPS)
	EnvK8sClientBurst = "ARGOCD_K8S_CLIENT_BURST"

	// EnvK8sClientMaxIdleConnections is the number of max idle connections in K8s REST client HTTP transport (default: 500)
	EnvK8sClientMaxIdleConnections = "ARGOCD_K8S_CLIENT_MAX_IDLE_CONNECTIONS"
)

// Constants associated with the Cluster API
var (

	// K8sClientConfigQPS controls the QPS to be used in K8s REST client configs
	K8sClientConfigQPS float32 = 50

	// K8sClientConfigBurst controls the burst to be used in K8s REST client configs
	K8sClientConfigBurst int = 100

	// K8sMaxIdleConnections controls the number of max idle connections in K8s REST client HTTP transport
	K8sMaxIdleConnections = 500
)

func init() {
	if envQPS := os.Getenv(EnvK8sClientQPS); envQPS != "" {
		if qps, err := strconv.ParseFloat(envQPS, 32); err != nil {
			K8sClientConfigQPS = float32(qps)
		}
	}
	if envBurst := os.Getenv(EnvK8sClientBurst); envBurst != "" {
		if burst, err := strconv.Atoi(envBurst); err != nil {
			K8sClientConfigBurst = burst
		}
	} else {
		K8sClientConfigBurst = 2 * int(K8sClientConfigQPS)
	}

	if envMaxConn := os.Getenv(EnvK8sClientMaxIdleConnections); envMaxConn != "" {
		if maxConn, err := strconv.Atoi(envMaxConn); err != nil {
			K8sMaxIdleConnections = maxConn
		}
	}

}
