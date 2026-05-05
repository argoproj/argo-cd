package apiclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/util/env"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/timeout"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	grpc_util "github.com/argoproj/argo-cd/v3/util/grpc"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

// MaxGRPCMessageSize contains max grpc message size
var MaxGRPCMessageSize = env.ParseNumFromEnv(common.EnvGRPCMaxSizeMB, 100, 0, math.MaxInt32) * 1024 * 1024

// TLSConfiguration describes parameters for TLS configuration to be used by a repo server API client
type TLSConfiguration struct {
	// Whether to disable TLS for connections
	DisableTLS bool
	// Whether to enforce strict validation of TLS certificates
	StrictValidation bool
	// List of certificates to validate the peer against (if StrictCerts is true)
	Certificates *x509.CertPool
	// ClientCertFile is the path to the client certificate file
	ClientCertFile string
	// ClientCertKeyFile is the path to the client certificate key file
	ClientCertKeyFile string
	// ClientCertificates are the client certificates to be used for TLS
	ClientCertificates []tls.Certificate
}

// Clientset represents repository server api clients
type Clientset interface {
	NewRepoServerClient() (utilio.Closer, RepoServerServiceClient, error)
}

type clientSet struct {
	address        string
	timeoutSeconds int
	tlsConfig      TLSConfiguration
}

type certCacheEntry struct {
	cert tls.Certificate
}

var clientCertCache = struct {
	mu      sync.Mutex
	entries map[string]certCacheEntry
}{
	entries: map[string]certCacheEntry{},
}

// ResetClientCertCache clears the client certificate cache.
// Intended for tests.
func ResetClientCertCache() {
	clientCertCache.mu.Lock()
	defer clientCertCache.mu.Unlock()
	clear(clientCertCache.entries)
}

func getClientCertFromCache(certFile, keyFile string) (tls.Certificate, error) {
	cacheKey := certFile + "\x00" + keyFile

	clientCertCache.mu.Lock()
	if entry, ok := clientCertCache.entries[cacheKey]; ok {
		clientCertCache.mu.Unlock()
		return entry.cert, nil
	}
	clientCertCache.mu.Unlock()

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return tls.Certificate{}, err
	}

	clientCertCache.mu.Lock()
	if entry, ok := clientCertCache.entries[cacheKey]; ok {
		clientCertCache.mu.Unlock()
		return entry.cert, nil
	}
	clientCertCache.entries[cacheKey] = certCacheEntry{cert: cert}
	clientCertCache.mu.Unlock()

	return cert, nil
}

func (c *clientSet) NewRepoServerClient() (utilio.Closer, RepoServerServiceClient, error) {
	conn, err := NewConnection(c.address, c.timeoutSeconds, &c.tlsConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open a new connection to repo server: %w", err)
	}
	return conn, NewRepoServerServiceClient(conn), nil
}

func NewConnection(address string, timeoutSeconds int, tlsConfig *TLSConfiguration) (*grpc.ClientConn, error) {
	retryOpts := []grpc_retry.CallOption{
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffLinear(1000 * time.Millisecond)),
	}
	unaryInterceptors := []grpc.UnaryClientInterceptor{grpc_retry.UnaryClientInterceptor(retryOpts...)}
	if timeoutSeconds > 0 {
		unaryInterceptors = append(unaryInterceptors, timeout.UnaryClientInterceptor(time.Duration(timeoutSeconds)*time.Second))
	}
	opts := []grpc.DialOption{
		grpc.WithStreamInterceptor(grpc_util.RetryOnlyForServerStreamInterceptor(retryOpts...)),
		grpc.WithChainUnaryInterceptor(unaryInterceptors...),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxGRPCMessageSize), grpc.MaxCallSendMsgSize(MaxGRPCMessageSize)),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	}

	if tlsConfig.DisableTLS {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		tlsC := &tls.Config{}
		if !tlsConfig.StrictValidation {
			tlsC.InsecureSkipVerify = true
		} else {
			tlsC.RootCAs = tlsConfig.Certificates
		}
		if tlsConfig.ClientCertFile != "" && tlsConfig.ClientCertKeyFile != "" {
			cert, err := getClientCertFromCache(tlsConfig.ClientCertFile, tlsConfig.ClientCertKeyFile)
			if err != nil {
				return nil, fmt.Errorf("failed to load client certificate: %w", err)
			}
			tlsC.Certificates = []tls.Certificate{cert}
		} else if len(tlsConfig.ClientCertificates) > 0 {
			tlsC.Certificates = tlsConfig.ClientCertificates
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsC)))
	}

	conn, err := grpc.NewClient(address, opts...)
	if err != nil {
		log.Errorf("Unable to connect to repository service with address %s", address)
		return nil, err
	}
	return conn, nil
}

// NewRepoServerClientset creates new instance of repo server Clientset
func NewRepoServerClientset(address string, timeoutSeconds int, tlsConfig TLSConfiguration) Clientset {
	return &clientSet{address: address, timeoutSeconds: timeoutSeconds, tlsConfig: tlsConfig}
}
