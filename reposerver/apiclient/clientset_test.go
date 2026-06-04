package apiclient_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	utilstls "github.com/argoproj/argo-cd/v3/util/tls"

	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient/mocks"
)

func TestNewRepoServerClient_CorrectClientReturned(t *testing.T) {
	t.Parallel()
	mockClientset := &mocks.Clientset{
		RepoServerServiceClient: &mocks.RepoServerServiceClient{},
	}

	closer, client, err := mockClientset.NewRepoServerClient()

	require.NoError(t, err)
	assert.NotNil(t, closer)
	assert.NotNil(t, client)
	assert.Equal(t, mockClientset.RepoServerServiceClient, client)
}

func TestNewRepoServerClientset_InvalidInput(t *testing.T) {
	t.Parallel()
	// Call the function with invalid inputs
	invalidClientset := apiclient.NewRepoServerClientset("", -1, apiclient.TLSConfiguration{})

	assert.NotNil(t, invalidClientset)
	assert.Implements(t, (*apiclient.Clientset)(nil), invalidClientset)
}

func TestNewRepoServerClientset_SuccessfulConnection(t *testing.T) {
	t.Parallel()
	// Call the function with valid inputs
	clientset := apiclient.NewRepoServerClientset("localhost:8080", 1, apiclient.TLSConfiguration{})

	assert.NotNil(t, clientset)
	assert.Implements(t, (*apiclient.Clientset)(nil), clientset)
}

func TestNewRepoServerClientset_SuccessfulConnectionWithTLS(t *testing.T) {
	t.Parallel()
	// Call the function with valid inputs
	clientset := apiclient.NewRepoServerClientset("localhost:8080", 1, apiclient.TLSConfiguration{
		DisableTLS:       false,
		StrictValidation: true,
		Certificates:     nil,
	})

	assert.NotNil(t, clientset)
	assert.Implements(t, (*apiclient.Clientset)(nil), clientset)
}

func TestNewConnection_TLSWithStrictValidation(t *testing.T) {
	t.Parallel()
	tlsConfig := apiclient.TLSConfiguration{
		DisableTLS:       false,
		StrictValidation: true,
		Certificates:     nil,
	}

	conn, err := apiclient.NewConnection("example.com:443", 10, &tlsConfig)

	require.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestNewConnection_TLSWithStrictValidationAndCertificates(t *testing.T) {
	t.Parallel()
	tlsConfig := apiclient.TLSConfiguration{
		DisableTLS:       false,
		StrictValidation: true,
		Certificates:     nil,
	}

	conn, err := apiclient.NewConnection("example.com:443", 10, &tlsConfig)

	require.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestNewConnection_InsecureConnection(t *testing.T) {
	t.Parallel()
	// Create a TLS configuration with TLS disabled
	tlsConfig := apiclient.TLSConfiguration{
		DisableTLS:       true,
		StrictValidation: false,
		Certificates:     nil,
	}

	conn, err := apiclient.NewConnection("example.com:80", 10, &tlsConfig)

	require.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestNewConnection_InMemoryCertificates(t *testing.T) {
	tlsConfig := apiclient.TLSConfiguration{
		DisableTLS:         false,
		StrictValidation:   false,
		ClientCertificates: []tls.Certificate{{}},
	}

	conn, err := apiclient.NewConnection("example.com:443", 10, &tlsConfig)

	require.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestNewConnection_ServesFromCacheWhenFilesUnchanged(t *testing.T) {
	t.Cleanup(apiclient.ResetClientCertCache)
	apiclient.ResetClientCertCache()

	certFile := filepath.Join("..", "..", "util", "tls", "testdata", "valid_tls.crt")
	keyFile := filepath.Join("..", "..", "util", "tls", "testdata", "valid_tls.key")

	tempCertFile := filepath.Join(t.TempDir(), "client.crt")
	tempKeyFile := filepath.Join(t.TempDir(), "client.key")

	certBytes, err := os.ReadFile(certFile)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tempCertFile, certBytes, 0o600))

	keyBytes, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tempKeyFile, keyBytes, 0o600))

	tlsConfig := apiclient.TLSConfiguration{
		StrictValidation:  false,
		ClientCertFile:    tempCertFile,
		ClientCertKeyFile: tempKeyFile,
	}

	// First call: loads from disk and populates the cache.
	conn, err := apiclient.NewConnection("example.com:443", 10, &tlsConfig)
	require.NoError(t, err)
	assert.NotNil(t, conn)

	// Second call: files are untouched, mtime is unchanged — cert must be served from the cache without re-reading disk.
	conn, err = apiclient.NewConnection("example.com:443", 10, &tlsConfig)
	require.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestNewConnection_ReloadsClientCertificateWhenFileChanges(t *testing.T) {
	t.Cleanup(apiclient.ResetClientCertCache)
	apiclient.ResetClientCertCache()

	certFile := filepath.Join("..", "..", "util", "tls", "testdata", "valid_tls.crt")
	keyFile := filepath.Join("..", "..", "util", "tls", "testdata", "valid_tls.key")

	tempCertFile := filepath.Join(t.TempDir(), "client.crt")
	tempKeyFile := filepath.Join(t.TempDir(), "client.key")

	certBytes, err := os.ReadFile(certFile)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tempCertFile, certBytes, 0o600))

	keyBytes, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tempKeyFile, keyBytes, 0o600))

	tlsConfig := apiclient.TLSConfiguration{
		StrictValidation:  false,
		ClientCertFile:    tempCertFile,
		ClientCertKeyFile: tempKeyFile,
	}

	// First call: loads and caches the cert.
	conn, err := apiclient.NewConnection("example.com:443", 10, &tlsConfig)
	require.NoError(t, err)
	assert.NotNil(t, conn)

	// Simulate cert rotation: overwrite with fresh (still valid) cert bytes.
	// Writing updates the mtime, which is enough to trigger a reload.
	require.NoError(t, os.WriteFile(tempCertFile, certBytes, 0o600))
	require.NoError(t, os.WriteFile(tempKeyFile, keyBytes, 0o600))

	// Second call: mtime changed → cache invalidated → reloads from disk.
	// The new cert is valid, so the call must succeed.
	conn, err = apiclient.NewConnection("example.com:443", 10, &tlsConfig)
	require.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestNewConnection_ErrorWhenRotatedCertIsInvalid(t *testing.T) {
	t.Cleanup(apiclient.ResetClientCertCache)
	apiclient.ResetClientCertCache()

	certFile := filepath.Join("..", "..", "util", "tls", "testdata", "valid_tls.crt")
	keyFile := filepath.Join("..", "..", "util", "tls", "testdata", "valid_tls.key")

	tempCertFile := filepath.Join(t.TempDir(), "client.crt")
	tempKeyFile := filepath.Join(t.TempDir(), "client.key")

	certBytes, err := os.ReadFile(certFile)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tempCertFile, certBytes, 0o600))

	keyBytes, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tempKeyFile, keyBytes, 0o600))

	tlsConfig := apiclient.TLSConfiguration{
		StrictValidation:  false,
		ClientCertFile:    tempCertFile,
		ClientCertKeyFile: tempKeyFile,
	}

	// First call: succeeds and populates the cache.
	conn, err := apiclient.NewConnection("example.com:443", 10, &tlsConfig)
	require.NoError(t, err)
	assert.NotNil(t, conn)

	require.NoError(t, os.WriteFile(tempCertFile, []byte("not a valid certificate"), 0o600))
	require.NoError(t, os.WriteFile(tempKeyFile, []byte("not a valid key"), 0o600))

	// Second call: mtime changed → reload attempted → parse fails → error returned.
	// The stale (valid) cached cert must NOT be silently served.
	_, err = apiclient.NewConnection("example.com:443", 10, &tlsConfig)
	require.Error(t, err,
		"a corrupt cert file must cause an error after cache invalidation, "+
			"not silently serve the stale cached cert")
	assert.Contains(t, err.Error(), "failed to load client certificate",
		"error must identify the cert load as the failure site")
}

func TestNewConnection_DoesNotCacheClientCertificateLoadError(t *testing.T) {
	t.Cleanup(apiclient.ResetClientCertCache)
	apiclient.ResetClientCertCache()

	tempDir := t.TempDir()
	tempCertFile := filepath.Join(tempDir, "client.crt")
	tempKeyFile := filepath.Join(tempDir, "client.key")

	tlsConfig := apiclient.TLSConfiguration{
		StrictValidation:  false,
		ClientCertFile:    tempCertFile,
		ClientCertKeyFile: tempKeyFile,
	}

	_, err := apiclient.NewConnection("example.com:443", 10, &tlsConfig)
	require.Error(t, err)

	certFile := filepath.Join("..", "..", "util", "tls", "testdata", "valid_tls.crt")
	keyFile := filepath.Join("..", "..", "util", "tls", "testdata", "valid_tls.key")

	certBytes, err := os.ReadFile(certFile)
	require.NoError(t, err)
	err = os.WriteFile(tempCertFile, certBytes, 0o600)
	require.NoError(t, err)

	keyBytes, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	err = os.WriteFile(tempKeyFile, keyBytes, 0o600)
	require.NoError(t, err)

	conn, err := apiclient.NewConnection("example.com:443", 10, &tlsConfig)
	require.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestMTLSIntegration_NoClientCert_IsRejected(t *testing.T) {
	t.Parallel()

	_, parsedClientCert := generateClientCA(t, "test-client")

	pool := x509.NewCertPool()
	pool.AddCert(parsedClientCert)
	fixture := newMTLSServer(t, pool)

	conn, err := apiclient.NewConnection(fixture.addr, 5, &apiclient.TLSConfiguration{
		StrictValidation: false,
	})
	require.NoError(t, err, "NewConnection is lazy — dial error surfaces on first RPC")
	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {
			t.Logf("failed to close connection: %v", err)
		}
	}(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn.Connect()
	for {
		st := conn.GetState()
		if st == connectivity.Ready || st == connectivity.TransientFailure || st == connectivity.Shutdown {
			break
		}
		if !conn.WaitForStateChange(ctx, st) {
			break
		}
	}

	err = healthCheck(t, conn)
	require.Error(t, err, "the server enforces mTLS; a client without a certificate must be rejected")
	assert.ErrorContains(t, err, "certificate", "rejection must be a TLS certificate error, not an unrelated failure")
}

func TestMTLSIntegration_ValidClientCert_IsAccepted(t *testing.T) {
	t.Parallel()
	t.Cleanup(apiclient.ResetClientCertCache)

	clientCert, parsedClientCert := generateClientCA(t, "test-client")

	pool := x509.NewCertPool()
	pool.AddCert(parsedClientCert)
	fixture := newMTLSServer(t, pool)

	conn, err := apiclient.NewConnection(fixture.addr, 5, &apiclient.TLSConfiguration{
		StrictValidation:   false,
		ClientCertificates: []tls.Certificate{*clientCert},
	})
	require.NoError(t, err)
	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {
			t.Logf("failed to close connection: %v", err)
		}
	}(conn)

	require.NoError(t, healthCheck(t, conn), "client presenting a cert trusted by the server's ClientCAs must succeed")
}

func TestMTLSIntegration_UntrustedClientCert_IsRejected(t *testing.T) {
	t.Parallel()

	_, trustedParsed := generateClientCA(t, "trusted-ca")
	trustedPool := x509.NewCertPool()
	trustedPool.AddCert(trustedParsed)
	fixture := newMTLSServer(t, trustedPool)

	untrustedCert, _ := generateClientCA(t, "untrusted-client")

	conn, err := apiclient.NewConnection(fixture.addr, 5, &apiclient.TLSConfiguration{
		StrictValidation:   false,
		ClientCertificates: []tls.Certificate{*untrustedCert},
	})
	require.NoError(t, err, "NewConnection is lazy")
	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {
			t.Logf("failed to close connection: %v", err)
		}
	}(conn)

	require.Error(t, healthCheck(t, conn), "a cert signed by an unknown CA must be rejected by the server")
}

func TestMTLSIntegration_HealthCheckEphemeralCert_IsAccepted(t *testing.T) {
	t.Parallel()

	hcCert, err := utilstls.GenerateHealthCheckClientCert()
	require.NoError(t, err, "GenerateHealthCheckClientCert")

	parsedHCCert, err := x509.ParseCertificate(hcCert.Certificate[0])
	require.NoError(t, err)

	pool := x509.NewCertPool()
	pool.AddCert(parsedHCCert)
	fixture := newMTLSServer(t, pool)

	conn, err := apiclient.NewConnection(fixture.addr, 5, &apiclient.TLSConfiguration{
		StrictValidation:   false, // liveness probe skips server cert verification
		ClientCertificates: []tls.Certificate{*hcCert},
	})
	require.NoError(t, err)
	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {
			t.Logf("failed to close connection: %v", err)
		}
	}(conn)

	require.NoError(t, healthCheck(t, conn), "liveness probe using the ephemeral health-check cert must be accepted")
}

// TestMTLSIntegration_DisableTLS_PlaintextConnection verifies that a server
// running without TLS accepts a plaintext client, establishing the baseline
// that DisableTLS=true on the client side still works after this PR.
func TestMTLSIntegration_DisableTLS_PlaintextConnection(t *testing.T) {
	t.Parallel()
	lis, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer() // no TLS credentials
	hsvc := health.NewServer()
	hsvc.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(srv, hsvc)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := apiclient.NewConnection(lis.Addr().String(), 5, &apiclient.TLSConfiguration{
		DisableTLS: true,
	})
	require.NoError(t, err)
	defer conn.Close()

	err = healthCheck(t, conn)
	require.NoError(t, err, "plaintext connection to a plaintext server must succeed")
}

// TestMTLSIntegration_StrictValidation_CertificatesPoolTakesPrecedence
// exercises the buildTLSClientConfig fix: when Certificates is non-nil,
// InsecureSkipVerify must be false even if StrictValidation=false in the
// struct.  This is the `strictValidation := tlsConfig.StrictValidation ||
// tlsConfig.Certificates != nil` guard in buildTLSClientConfig.
//
// The test verifies observable behaviour: the connection succeeds only when
// the client presents the correct CA pool (i.e. server cert verification is
// actually being performed, not skipped).
func TestMTLSIntegration_StrictValidation_CertificatesPoolTakesPrecedence(t *testing.T) {
	t.Parallel()

	clientCert, parsedClientCert := generateClientCA(t, "test-client-strict")
	clientPool := x509.NewCertPool()
	clientPool.AddCert(parsedClientCert)
	fixture := newMTLSServer(t, clientPool)

	// We need the client to verify the server's certificate, so we must tell
	// it which CA signed the server cert.  The server is self-signed, so we
	// extract its cert and build a server-CA pool from it.
	serverTLSState, err := extractServerCert(t, fixture.addr, clientCert)
	require.NoError(t, err)
	serverCAParsed, err := x509.ParseCertificate(serverTLSState.Certificate[0])
	require.NoError(t, err)
	serverCAPool := x509.NewCertPool()
	serverCAPool.AddCert(serverCAParsed)

	// StrictValidation=false BUT Certificates is non-nil.
	// buildTLSClientConfig must treat this as strict=true (the fix).
	conn, err := apiclient.NewConnection(fixture.addr, 5, &apiclient.TLSConfiguration{
		StrictValidation:   false,        // struct field says "not strict" …
		Certificates:       serverCAPool, // … but a pool is present, so strict wins
		ClientCertificates: []tls.Certificate{*clientCert},
	})
	require.NoError(t, err)
	defer conn.Close()

	err = healthCheck(t, conn)
	require.NoError(t, err,
		"when Certificates pool is set, server cert must be verified and the "+
			"connection must succeed when the pool contains the server's CA")
}

func extractServerCert(t *testing.T, addr string, clientCert *tls.Certificate) (*tls.Certificate, error) {
	t.Helper()
	cfg := &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // intentional: only used to fetch the cert bytes
		Certificates:       []tls.Certificate{*clientCert},
	}
	d := &tls.Dialer{Config: cfg}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	tlsConnection, ok := conn.(*tls.Conn)
	if !ok {
		t.Fatal("expected *tls.Conn from tls.Dialer")
	}
	state := tlsConnection.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		t.Fatal("server sent no certificate")
	}
	raw := state.PeerCertificates[0].Raw
	tlsCert := &tls.Certificate{Certificate: [][]byte{raw}}
	return tlsCert, nil
}

type mTLSServerFixture struct {
	addr   string
	server *grpc.Server //nolint:unused // for some reason this linter complains
}

func newMTLSServer(t *testing.T, clientCAs *x509.CertPool) *mTLSServerFixture {
	t.Helper()

	serverCert, err := utilstls.GenerateX509KeyPair(utilstls.CertOptions{
		Hosts:        []string{"localhost", "127.0.0.1"},
		Organization: "Argo CD Integration Test Server",
		IsCA:         false,
		ECDSACurve:   "P256",
		ValidFor:     time.Hour,
	})
	require.NoError(t, err, "generating server certificate")

	serverTLSCfg := &tls.Config{
		Certificates: []tls.Certificate{*serverCert},
		ClientCAs:    clientCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}

	lis, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err, "binding test listener")

	srv := grpc.NewServer(grpc.Creds(credentials.NewTLS(serverTLSCfg)))
	hsvc := health.NewServer()
	hsvc.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(srv, hsvc)

	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	return &mTLSServerFixture{addr: lis.Addr().String(), server: srv}
}

func healthCheck(t *testing.T, conn *grpc.ClientConn) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		return status.Errorf(codes.Unavailable, "server status: %v", resp.GetStatus())
	}
	return nil
}

func generateClientCA(t *testing.T, cn string) (*tls.Certificate, *x509.Certificate) {
	t.Helper()
	cert, err := utilstls.GenerateX509KeyPair(utilstls.CertOptions{
		Hosts:        []string{cn},
		Organization: "Argo CD Integration Test",
		IsCA:         true,
		ECDSACurve:   "P256",
		ValidFor:     time.Hour,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err)
	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)
	return cert, parsed
}
