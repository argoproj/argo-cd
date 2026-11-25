package v1alpha1

import (
	"math"
	"time"

	"github.com/argoproj/argo-cd/v3/util/env"
)

const (

	// EnvVarFakeInClusterConfig is an environment variable to fake an in-cluster RESTConfig using
	// the current kubectl context (for development purposes)
	EnvVarFakeInClusterConfig = "ARGOCD_FAKE_IN_CLUSTER"

	// EnvK8sClientQPS is the QPS value used for the kubernetes client (default: 50)
	// Set to -1 to disable client-side rate limiting and rely on API Priority and Fairness
	EnvK8sClientQPS = "ARGOCD_K8S_CLIENT_QPS"

	// EnvK8sClientBurst is the burst value used for the kubernetes client (default: twice the client QPS)
	EnvK8sClientBurst = "ARGOCD_K8S_CLIENT_BURST"

	// EnvK8sClientMaxIdleConnections is the number of max idle connections in K8s REST client HTTP transport (default: 500)
	EnvK8sClientMaxIdleConnections = "ARGOCD_K8S_CLIENT_MAX_IDLE_CONNECTIONS"

	// EnvK8sTCPTimeout is the duration for TCP timeouts when communicating with K8s API servers
	EnvK8sTCPTimeout = "ARGOCD_K8S_TCP_TIMEOUT"

	// EnvK8sTCPKeepAlive is the interval for TCP keep alive probes to be sent when communicating with K8s API servers
	EnvK8sTCPKeepAlive = "ARGOCD_K8S_TCP_KEEPALIVE"

	// EnvK8sTLSHandshakeTimeout is the duration for TLS handshake timeouts when establishing connections to K8s API servers
	EnvK8sTLSHandshakeTimeout = "ARGOCD_K8S_TLS_HANDSHAKE_TIMEOUT"

	// EnvK8sTCPIdleConnTimeout is the duration when idle TCP connection to the K8s API servers should timeout
	EnvK8sTCPIdleConnTimeout = "ARGOCD_K8S_TCP_IDLE_TIMEOUT"

	// EnvK8sServerSideTimeout is the duration for the server side timeout for each API request
	EnvK8sServerSideTimeout = "ARGOCD_K8S_SERVER_SIDE_TIMEOUT"
)

// Configuration variables associated with the Cluster API
var (

	// K8sClientConfigQPS controls the QPS to be used in K8s REST client configs
	// A negative value (-1) disables client-side rate limiting: client-go only creates
	// a token bucket rate limiter when QPS > 0
	K8sClientConfigQPS float32 = k8sClientQPSFromEnv()

	// K8sClientConfigBurst controls the burst to be used in K8s REST client configs
	K8sClientConfigBurst int = k8sClientBurstFromEnv(K8sClientConfigQPS)

	// K8sMaxIdleConnections controls the number of max idle connections in K8s REST client HTTP transport
	K8sMaxIdleConnections = env.ParseNumFromEnv(EnvK8sClientMaxIdleConnections, 500, 0, math.MaxInt32)

	// K8sTLSHandshakeTimeout defines the maximum duration to wait for a TLS handshake to complete
	K8sTLSHandshakeTimeout = env.ParseDurationFromEnv(EnvK8sTLSHandshakeTimeout, 10*time.Second, 0, math.MaxInt32*time.Second)

	// K8sTCPTimeout defines the TCP timeout to use when performing K8s API requests
	K8sTCPTimeout = env.ParseDurationFromEnv(EnvK8sTCPTimeout, 30*time.Second, 0, math.MaxInt32*time.Second)

	// K8sTCPKeepAlive defines the interval for sending TCP keep alive to K8s API server
	K8sTCPKeepAlive = env.ParseDurationFromEnv(EnvK8sTCPKeepAlive, 30*time.Second, 0, math.MaxInt32*time.Second)

	// K8sTCPIdleConnTimeout defines the duration for keeping idle TCP connections to the K8s API server
	K8sTCPIdleConnTimeout = env.ParseDurationFromEnv(EnvK8sTCPIdleConnTimeout, 5*time.Minute, 0, math.MaxInt32*time.Second)

	// K8sServerSideTimeout defines which server side timeout to send with each API request
	K8sServerSideTimeout = env.ParseDurationFromEnv(EnvK8sServerSideTimeout, 0, 0, math.MaxInt32*time.Second)
)

// k8sClientQPSFromEnv reads EnvK8sClientQPS. Values from -1 upwards are accepted; -1 disables
// client-side rate limiting. 0 is still passed through and results in client-go's own default
func k8sClientQPSFromEnv() float32 {
	return env.ParseFloatFromEnv(EnvK8sClientQPS, 50, -1, math.MaxFloat32)
}

// k8sClientBurstFromEnv reads EnvK8sClientBurst, defaulting to twice the QPS. When client-side
// rate limiting is disabled (negative QPS) burst is unused, so it defaults to 0
func k8sClientBurstFromEnv(qps float32) int {
	defaultBurst := 0
	if qps > 0 {
		defaultBurst = int(2 * qps)
	}
	return env.ParseNumFromEnv(EnvK8sClientBurst, defaultBurst, 0, math.MaxInt32)
}
