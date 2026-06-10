package apiclient

import (
	"crypto/tls"
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/util/env"
	utiltls "github.com/argoproj/argo-cd/v3/util/tls"

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

// Clientset represents repository server api clients
type Clientset interface {
	NewRepoServerClient() (utilio.Closer, RepoServerServiceClient, error)
}

type clientSet struct {
	address        string
	timeoutSeconds int
	tlsConfig      utiltls.Configuration
}

type clientCertEntry struct {
	cert        tls.Certificate
	certModTime time.Time
	keyModTime  time.Time
}

var clientCertCache = struct {
	mu      sync.RWMutex
	entries map[string]*clientCertEntry
}{
	entries: map[string]*clientCertEntry{},
}

// ResetClientCertCache clears the client certificate cache. // Intended for tests.
func ResetClientCertCache() {
	clientCertCache.mu.Lock()
	defer clientCertCache.mu.Unlock()
	clear(clientCertCache.entries)
}

func getClientCertFromCache(certFile, keyFile string) (tls.Certificate, error) {
	cacheKey := certFile + "\x00" + keyFile

	// Stat both files before acquiring the write lock.
	// os.Stat is cheap; we do it outside the lock to minimise contention.
	certInfo, err := os.Stat(certFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("cannot stat client cert file %s: %w", certFile, err)
	}
	keyInfo, err := os.Stat(keyFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("cannot stat client cert key file %s: %w", keyFile, err)
	}

	// Fast path: read lock, return cached cert if still fresh.
	clientCertCache.mu.RLock()
	if entry, ok := clientCertCache.entries[cacheKey]; ok {
		if !certInfo.ModTime().After(entry.certModTime) && !keyInfo.ModTime().After(entry.keyModTime) {
			cert := entry.cert
			clientCertCache.mu.RUnlock()
			return cert, nil
		}
	}
	clientCertCache.mu.RUnlock()

	// Slow path: files changed or not yet cached — reload from disk.
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to load client certificate: %w", err)
	}

	clientCertCache.mu.Lock()
	// Double-check: another goroutine may have loaded between our RUnlock and Lock.
	if entry, ok := clientCertCache.entries[cacheKey]; ok {
		if !certInfo.ModTime().After(entry.certModTime) && !keyInfo.ModTime().After(entry.keyModTime) {
			cert := entry.cert
			clientCertCache.mu.Unlock()
			return cert, nil
		}
	}
	clientCertCache.entries[cacheKey] = &clientCertEntry{
		cert:        cert,
		certModTime: certInfo.ModTime(),
		keyModTime:  keyInfo.ModTime(),
	}
	clientCertCache.mu.Unlock()

	log.Infof("Client certificate reloaded from %s (mtime changed)", certFile)
	return cert, nil
}

func (c *clientSet) NewRepoServerClient() (utilio.Closer, RepoServerServiceClient, error) {
	conn, err := NewConnection(c.address, c.timeoutSeconds, &c.tlsConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open a new connection to repo server: %w", err)
	}
	return conn, NewRepoServerServiceClient(conn), nil
}

func NewConnection(address string, timeoutSeconds int, tlsConfig *utiltls.Configuration) (*grpc.ClientConn, error) {
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
		tlsC, err := buildTLSClientConfig(tlsConfig)
		if err != nil {
			return nil, err
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
func NewRepoServerClientset(address string, timeoutSeconds int, tlsConfig utiltls.Configuration) Clientset {
	return &clientSet{
		address:        address,
		timeoutSeconds: timeoutSeconds,
		tlsConfig:      tlsConfig,
	}
}

func buildTLSClientConfig(tlsConfig *utiltls.Configuration) (*tls.Config, error) {
	tlsC := &tls.Config{}

	strictValidation := tlsConfig.StrictValidation || tlsConfig.Certificates != nil
	if !strictValidation {
		tlsC.InsecureSkipVerify = true
	} else {
		tlsC.RootCAs = tlsConfig.Certificates
	}

	if tlsConfig.ClientCertFile != "" && tlsConfig.ClientCertKeyFile != "" {
		certFile := tlsConfig.ClientCertFile
		keyFile := tlsConfig.ClientCertKeyFile

		if _, err := getClientCertFromCache(certFile, keyFile); err != nil {
			return nil, fmt.Errorf("failed to load initial client certificate: %w", err)
		}

		tlsC.GetClientCertificate = func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			cert, err := getClientCertFromCache(certFile, keyFile)
			if err != nil {
				return nil, err
			}
			return &cert, nil
		}
	} else if len(tlsConfig.ClientCertificates) > 0 {
		tlsC.Certificates = tlsConfig.ClientCertificates
	}

	return tlsC, nil
}
