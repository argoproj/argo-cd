package v1alpha1

import (
	"math"
	"time"

	"github.com/argoproj/argo-cd/v2/util/env"
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

	// EnvK8sTCPTimeout is the duration for TCP timeouts when communicating with K8s API servers
	EnvK8sTCPTimeout = "ARGOCD_K8S_TCP_TIMEOUT"

	// EnvK8sTCPKeepalive is the interval for TCP keep alive probes to be sent when communicating with K8s API servers
	EnvK8sTCPKeepAlive = "ARGOCD_K8S_TCP_KEEPALIVE"

	// EnvK8sTLSHandshakeTimeout is the duration for TLS handshake timeouts when establishing connections to K8s API servers
	EnvK8sTLSHandshakeTimeout = "ARGOCD_K8S_TLS_HANDSHAKE_TIMEOUT"

	// EnvK8sTCPIdleConnTimeout is the duration when idle TCP connection to the K8s API servers should timeout
	EnvK8sTCPIdleConnTimeout = "ARGOCD_K8S_TCP_IDLE_TIMEOUT"
)

// Constants associated with the Cluster API
var (

	// K8sClientConfigQPS controls the QPS to be used in K8s REST client configs
	K8sClientConfigQPS float32 = 50

	// K8sClientConfigBurst controls the burst to be used in K8s REST client configs
	K8sClientConfigBurst int = 100

	// K8sMaxIdleConnections controls the number of max idle connections in K8s REST client HTTP transport
	K8sMaxIdleConnections = 500

	// K8sTLSHandshakeTimeout defines the maximum duration to wait for a TLS handshake to complete
	K8sTLSHandshakeTimeout = 10 * time.Second

	// K8sTCPTimeout defines the TCP timeout to use when performing K8s API requests
	K8sTCPTimeout = 30 * time.Second

	// K8sTCPKeepAlive defines the interval for sending TCP keep alive to K8s API server
	K8sTCPKeepAlive = 30 * time.Second

	// K8sTCPIdleConnTimeout defines the duration for keeping idle TCP connections to the K8s API server
	K8sTCPIdleConnTimeout = 5 * time.Minute

	// K8sServerSideTimeout defines which server side timeout to send with each API request
	K8sServerSideTimeout = 32 * time.Second
)

func init() {
	K8sClientConfigQPS = env.ParseFloatFromEnv(EnvK8sClientQPS, K8sClientConfigQPS, 0, math.MaxFloat32)
	K8sClientConfigBurst = env.ParseNumFromEnv(EnvK8sClientBurst, int(2*K8sClientConfigQPS), 0, math.MaxInt32)
	K8sMaxIdleConnections = env.ParseNumFromEnv(EnvK8sClientMaxIdleConnections, K8sMaxIdleConnections, 0, math.MaxInt32)
	K8sTCPKeepAlive = env.ParseDurationFromEnv(EnvK8sTCPKeepAlive, K8sTCPKeepAlive, 0, math.MaxInt32)
	K8sServerSideTimeout = env.ParseDurationFromEnv(EnvK8sTCPTimeout, K8sServerSideTimeout, 0, math.MaxInt32)
	K8sTCPIdleConnTimeout = env.ParseDurationFromEnv(EnvK8sTCPIdleConnTimeout, K8sTCPIdleConnTimeout, 0, math.MaxInt32)
	K8sTLSHandshakeTimeout = env.ParseDurationFromEnv(EnvK8sTLSHandshakeTimeout, K8sTLSHandshakeTimeout, 0, math.MaxInt32)
	K8sTCPTimeout = env.ParseDurationFromEnv(EnvK8sTCPTimeout, K8sTCPTimeout, 0, math.MaxInt32)
}
