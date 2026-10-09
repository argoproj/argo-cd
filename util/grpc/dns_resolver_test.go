package grpc

import (
	"context"
	"errors"
	"net"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

// mockClientConn records resolver callbacks for verification.
type mockClientConn struct {
	mu        sync.Mutex
	states    []resolver.State
	errors    []error
	stateChan chan resolver.State
	errChan   chan error
}

func newMockClientConn() *mockClientConn {
	return &mockClientConn{
		stateChan: make(chan resolver.State, 10),
		errChan:   make(chan error, 10),
	}
}

func (m *mockClientConn) UpdateState(s resolver.State) error {
	m.mu.Lock()
	m.states = append(m.states, s)
	m.mu.Unlock()
	select {
	case m.stateChan <- s:
	default:
	}
	return nil
}

func (m *mockClientConn) ReportError(err error) {
	m.mu.Lock()
	m.errors = append(m.errors, err)
	m.mu.Unlock()
	select {
	case m.errChan <- err:
	default:
	}
}

func (m *mockClientConn) NewAddress(_ []resolver.Address) {
	panic("deprecated and unused")
}

func (m *mockClientConn) NewServiceConfig(_ string) {
	panic("deprecated and unused")
}

func (m *mockClientConn) ParseServiceConfig(_ string) *serviceconfig.ParseResult {
	return &serviceconfig.ParseResult{}
}

// failingClientConn injects errors into UpdateState for testing error handling.
type failingClientConn struct {
	resolver.ClientConn
	updateErr error
}

func (f *failingClientConn) UpdateState(resolver.State) error {
	return f.updateErr
}

// mockNetResolver records lookups and returns stubbed DNS responses.
type mockNetResolver struct {
	mu           sync.Mutex
	lookupHosts  []string
	hostResponse map[string][]string
	hostError    map[string]error
}

func newMockNetResolver() *mockNetResolver {
	return &mockNetResolver{
		hostResponse: make(map[string][]string),
		hostError:    make(map[string]error),
	}
}

func (m *mockNetResolver) LookupHost(_ context.Context, host string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lookupHosts = append(m.lookupHosts, host)

	if err, ok := m.hostError[host]; ok && err != nil {
		return nil, err
	}
	if ips, ok := m.hostResponse[host]; ok {
		return ips, nil
	}
	return []string{"192.0.2.1"}, nil
}

func TestCustomDNSResolver_Scheme(t *testing.T) {
	builder := NewDNSBuilder()
	assert.Equal(t, "dns", builder.Scheme())

	registered := resolver.Get("dns")
	require.NotNil(t, registered)
	assert.Equal(t, "dns", registered.Scheme())
}

func TestCustomDNSResolver_Build_IPLiteral_IPv4(t *testing.T) {
	builder := NewDNSBuilder()
	cc := newMockClientConn()

	target := resolver.Target{
		URL: url.URL{Path: "/127.0.0.1:8081"},
	}
	r, err := builder.Build(target, cc, resolver.BuildOptions{})
	require.NoError(t, err)
	defer r.Close()

	// Verify deadResolver ResolveNow is a no-op and doesn't panic
	r.ResolveNow(resolver.ResolveNowOptions{})

	select {
	case s := <-cc.stateChan:
		require.Len(t, s.Addresses, 1)
		assert.Equal(t, "127.0.0.1:8081", s.Addresses[0].Addr)
		require.Len(t, s.Endpoints, 1)
		assert.Equal(t, "127.0.0.1:8081", s.Endpoints[0].Addresses[0].Addr)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for state update")
	}
}

func TestCustomDNSResolver_Build_IPLiteral_IPv6(t *testing.T) {
	builder := NewDNSBuilder()
	cc := newMockClientConn()

	target := resolver.Target{
		URL: url.URL{Path: "/[::1]:9090"},
	}
	r, err := builder.Build(target, cc, resolver.BuildOptions{})
	require.NoError(t, err)
	defer r.Close()

	select {
	case s := <-cc.stateChan:
		require.Len(t, s.Addresses, 1)
		assert.Equal(t, "[::1]:9090", s.Addresses[0].Addr)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for state update")
	}
}

func TestCustomDNSResolver_Build_IPLiteral_UpdateStateError(t *testing.T) {
	builder := NewDNSBuilder()
	failingCC := &failingClientConn{
		ClientConn: newMockClientConn(),
		updateErr:  errors.New("update state failure"),
	}

	target := resolver.Target{
		URL: url.URL{Path: "/127.0.0.1:8081"},
	}
	r, err := builder.Build(target, failingCC, resolver.BuildOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update state failure")
	assert.Nil(t, r)
}

func TestCustomDNSResolver_Build_InvalidTarget(t *testing.T) {
	builder := NewDNSBuilder()
	cc := newMockClientConn()

	target := resolver.Target{
		URL: url.URL{Path: "/localhost:"},
	}
	r, err := builder.Build(target, cc, resolver.BuildOptions{})
	require.Error(t, err)
	assert.Nil(t, r)
}

func TestCustomDNSResolver_Build_CustomAuthority(t *testing.T) {
	t.Run("valid authority", func(t *testing.T) {
		mockNet := newMockNetResolver()
		origNewNetResolver := newNetResolver
		var capturedAuthority string
		newNetResolver = func(auth string) (netResolver, error) {
			capturedAuthority = auth
			return mockNet, nil
		}
		defer func() { newNetResolver = origNewNetResolver }()

		builder := NewDNSBuilder()
		cc := newMockClientConn()

		target := resolver.Target{
			URL: url.URL{Host: "8.8.8.8:53", Path: "/my-server:8080"},
		}
		r, err := builder.Build(target, cc, resolver.BuildOptions{})
		require.NoError(t, err)
		defer r.Close()

		assert.Equal(t, "8.8.8.8:53", capturedAuthority)
	})

	t.Run("invalid authority fails build", func(t *testing.T) {
		builder := NewDNSBuilder()
		cc := newMockClientConn()

		target := resolver.Target{
			URL: url.URL{Host: "invalid:authority:too:many:colons:", Path: "/my-server:8080"},
		}
		r, err := builder.Build(target, cc, resolver.BuildOptions{})
		require.Error(t, err)
		assert.Nil(t, r)
	})
}

func TestCustomDNSResolver_HostnameResolution(t *testing.T) {
	mockNet := newMockNetResolver()
	mockNet.hostResponse["argo-cd-repo-server"] = []string{"10.244.0.5", "10.244.0.6"}

	origNewNetResolver := newNetResolver
	newNetResolver = func(string) (netResolver, error) {
		return mockNet, nil
	}
	defer func() { newNetResolver = origNewNetResolver }()

	builder := NewDNSBuilder()
	cc := newMockClientConn()

	target := resolver.Target{
		URL: url.URL{Path: "/argo-cd-repo-server:8081"},
	}
	r, err := builder.Build(target, cc, resolver.BuildOptions{})
	require.NoError(t, err)
	defer r.Close()

	select {
	case s := <-cc.stateChan:
		require.Len(t, s.Addresses, 2)
		assert.Equal(t, "10.244.0.5:8081", s.Addresses[0].Addr)
		assert.Equal(t, "10.244.0.6:8081", s.Addresses[1].Addr)
		require.Len(t, s.Endpoints, 2)
		assert.Equal(t, "10.244.0.5:8081", s.Endpoints[0].Addresses[0].Addr)
		assert.Equal(t, "10.244.0.6:8081", s.Endpoints[1].Addresses[0].Addr)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for state update")
	}

	mockNet.mu.Lock()
	assert.Contains(t, mockNet.lookupHosts, "argo-cd-repo-server")
	mockNet.mu.Unlock()
}

func TestCustomDNSResolver_ResolveNow(t *testing.T) {
	origInterval := MinResolutionInterval
	MinResolutionInterval = 0
	defer func() { MinResolutionInterval = origInterval }()

	mockNet := newMockNetResolver()
	mockNet.hostResponse["my-service"] = []string{"192.0.2.1"}

	origNewNetResolver := newNetResolver
	newNetResolver = func(string) (netResolver, error) {
		return mockNet, nil
	}
	defer func() { newNetResolver = origNewNetResolver }()

	builder := NewDNSBuilder()
	cc := newMockClientConn()

	target := resolver.Target{
		URL: url.URL{Path: "/my-service:8080"},
	}
	r, err := builder.Build(target, cc, resolver.BuildOptions{})
	require.NoError(t, err)
	defer r.Close()

	// First resolution
	select {
	case s := <-cc.stateChan:
		require.Len(t, s.Addresses, 1)
		assert.Equal(t, "192.0.2.1:8080", s.Addresses[0].Addr)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first state update")
	}

	// Update mock IP and invoke ResolveNow twice to exercise the non-blocking channel send default branch
	mockNet.mu.Lock()
	mockNet.hostResponse["my-service"] = []string{"192.0.2.2"}
	mockNet.mu.Unlock()

	r.ResolveNow(resolver.ResolveNowOptions{})
	r.ResolveNow(resolver.ResolveNowOptions{})

	select {
	case s := <-cc.stateChan:
		require.Len(t, s.Addresses, 1)
		assert.Equal(t, "192.0.2.2:8080", s.Addresses[0].Addr)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ResolveNow state update")
	}
}

func TestCustomDNSResolver_LookupHost_InvalidIP(t *testing.T) {
	mockNet := newMockNetResolver()
	mockNet.hostResponse["invalid-ip-host"] = []string{"not-an-ip"}

	origNewNetResolver := newNetResolver
	newNetResolver = func(string) (netResolver, error) {
		return mockNet, nil
	}
	defer func() { newNetResolver = origNewNetResolver }()

	builder := NewDNSBuilder()
	cc := newMockClientConn()

	target := resolver.Target{
		URL: url.URL{Path: "/invalid-ip-host:8080"},
	}
	r, err := builder.Build(target, cc, resolver.BuildOptions{})
	require.NoError(t, err)
	defer r.Close()

	select {
	case err := <-cc.errChan:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error parsing IP address")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for error report")
	}
}

func TestCustomDNSResolver_ErrorHandling(t *testing.T) {
	mockNet := newMockNetResolver()
	mockNet.hostError["failing-host"] = errors.New("lookup failed")

	origNewNetResolver := newNetResolver
	newNetResolver = func(string) (netResolver, error) {
		return mockNet, nil
	}
	defer func() { newNetResolver = origNewNetResolver }()

	builder := NewDNSBuilder()
	cc := newMockClientConn()

	target := resolver.Target{
		URL: url.URL{Path: "/failing-host:8080"},
	}
	r, err := builder.Build(target, cc, resolver.BuildOptions{})
	require.NoError(t, err)
	defer r.Close()

	select {
	case err := <-cc.errChan:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "lookup failed")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for error report")
	}
}

type recordingBuilder struct {
	resolver.Builder
	buildCalled bool
	target      resolver.Target
}

func (b *recordingBuilder) Build(target resolver.Target, _ resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	b.buildCalled = true
	b.target = target
	return deadResolver{}, nil
}

func (b *recordingBuilder) Scheme() string {
	return "dns"
}

func TestCustomDNSResolver_EnvironmentVariable_Toggle(t *testing.T) {
	t.Run("DefaultEnabled", func(t *testing.T) {
		t.Setenv(EnvGRPCDisableCustomDNSResolver, "")
		assert.False(t, IsCustomDNSResolverDisabled())
	})

	t.Run("DisabledViaTrue", func(t *testing.T) {
		t.Setenv(EnvGRPCDisableCustomDNSResolver, "true")
		assert.True(t, IsCustomDNSResolverDisabled())
	})

	t.Run("DisabledViaFalse", func(t *testing.T) {
		t.Setenv(EnvGRPCDisableCustomDNSResolver, "false")
		assert.False(t, IsCustomDNSResolverDisabled())
	})

	t.Run("DisabledDelegation", func(t *testing.T) {
		t.Setenv(EnvGRPCDisableCustomDNSResolver, "true")
		stub := &recordingBuilder{}
		origDefault := defaultDNSBuilder
		defaultDNSBuilder = stub
		defer func() { defaultDNSBuilder = origDefault }()

		builder := NewDNSBuilder()
		cc := newMockClientConn()

		target := resolver.Target{
			URL: url.URL{Path: "/test-service:8081"},
		}
		r, err := builder.Build(target, cc, resolver.BuildOptions{})
		require.NoError(t, err)
		defer r.Close()

		assert.True(t, stub.buildCalled)
		assert.Equal(t, target, stub.target)
	})
}

func TestCustomDNSResolver_ParseTarget(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		defaultPort string
		wantHost    string
		wantPort    string
		wantErr     bool
	}{
		{
			name:        "empty target",
			target:      "",
			defaultPort: "443",
			wantErr:     true,
		},
		{
			name:        "ipv4 with port",
			target:      "192.0.2.1:8080",
			defaultPort: "443",
			wantHost:    "192.0.2.1",
			wantPort:    "8080",
		},
		{
			name:        "ipv4 without port",
			target:      "192.0.2.1",
			defaultPort: "443",
			wantHost:    "192.0.2.1",
			wantPort:    "443",
		},
		{
			name:        "hostname with port",
			target:      "argo-cd-repo-server:8081",
			defaultPort: "443",
			wantHost:    "argo-cd-repo-server",
			wantPort:    "8081",
		},
		{
			name:        "hostname without port",
			target:      "argo-cd-repo-server",
			defaultPort: "443",
			wantHost:    "argo-cd-repo-server",
			wantPort:    "443",
		},
		{
			name:        "ipv6 with brackets and port",
			target:      "[2001:db8::1]:8080",
			defaultPort: "443",
			wantHost:    "2001:db8::1",
			wantPort:    "8080",
		},
		{
			name:        "ipv6 without port",
			target:      "2001:db8::1",
			defaultPort: "443",
			wantHost:    "2001:db8::1",
			wantPort:    "443",
		},
		{
			name:        "ends with colon",
			target:      "localhost:",
			defaultPort: "443",
			wantErr:     true,
		},
		{
			name:        "empty host with port",
			target:      ":8080",
			defaultPort: "443",
			wantHost:    "localhost",
			wantPort:    "8080",
		},
		{
			name:        "multiple colons without brackets",
			target:      "foo:bar:baz",
			defaultPort: "443",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, p, err := parseTarget(tt.target, tt.defaultPort)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantHost, h)
				assert.Equal(t, tt.wantPort, p)
			}
		})
	}
}

func TestCustomDNSResolver_FormatIP(t *testing.T) {
	// IPv4
	ip4, err := formatIP("192.0.2.1")
	require.NoError(t, err)
	assert.Equal(t, "192.0.2.1", ip4)

	// IPv6
	ip6, err := formatIP("2001:db8::1")
	require.NoError(t, err)
	assert.Equal(t, "[2001:db8::1]", ip6)

	// Invalid IP
	_, err = formatIP("not-an-ip")
	require.Error(t, err)
}

func TestCustomDNSResolver_NewNetResolver(t *testing.T) {
	t.Run("default authority", func(t *testing.T) {
		res, err := newNetResolver("")
		require.NoError(t, err)
		assert.Equal(t, net.DefaultResolver, res)
	})

	t.Run("custom authority with dialer", func(t *testing.T) {
		res, err := newNetResolver("127.0.0.1:53")
		require.NoError(t, err)
		netRes, ok := res.(*net.Resolver)
		require.True(t, ok)
		assert.True(t, netRes.PreferGo)
		require.NotNil(t, netRes.Dial)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, dialErr := netRes.Dial(ctx, "tcp", "127.0.0.1:53")
		require.Error(t, dialErr)
	})

	t.Run("invalid authority", func(t *testing.T) {
		_, err := newNetResolver("invalid:authority:with:too:many:colons:")
		require.Error(t, err)
	})
}

func TestCustomDNSResolver_HandleDNSError(t *testing.T) {
	t.Run("non-timeout non-temporary dns error returns nil", func(t *testing.T) {
		dnsErr := &net.DNSError{
			Err:         "no such host",
			IsTimeout:   false,
			IsTemporary: false,
		}
		assert.NoError(t, handleDNSError(dnsErr))
	})

	t.Run("timeout dns error returns formatted error", func(t *testing.T) {
		dnsErr := &net.DNSError{
			Err:         "i/o timeout",
			IsTimeout:   true,
			IsTemporary: false,
		}
		err := handleDNSError(dnsErr)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dns: A record lookup error")
	})

	t.Run("generic error returns formatted error", func(t *testing.T) {
		err := handleDNSError(errors.New("network failure"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dns: A record lookup error: network failure")
	})

	t.Run("nil error returns nil", func(t *testing.T) {
		assert.NoError(t, handleDNSError(nil))
	})
}

func TestCustomDNSResolver_ExponentialBackoff(t *testing.T) {
	t.Run("attempt <= 0", func(t *testing.T) {
		d0 := exponentialBackoff(0)
		assert.Equal(t, 1*time.Second, d0)

		dNeg := exponentialBackoff(-1)
		assert.Equal(t, 1*time.Second, dNeg)
	})

	t.Run("attempt = 1", func(t *testing.T) {
		d := exponentialBackoff(1)
		// 1.6s +/- 20% jitter -> [1.28s, 1.92s]
		assert.GreaterOrEqual(t, d, time.Duration(1.2*float64(time.Second)))
		assert.LessOrEqual(t, d, time.Duration(2.0*float64(time.Second)))
	})

	t.Run("attempt = 2", func(t *testing.T) {
		d := exponentialBackoff(2)
		// 2.56s +/- 20% jitter -> [2.048s, 3.072s]
		assert.GreaterOrEqual(t, d, time.Duration(2.0*float64(time.Second)))
		assert.LessOrEqual(t, d, time.Duration(3.2*float64(time.Second)))
	})

	t.Run("attempt = 20 caps at maxDelay", func(t *testing.T) {
		d := exponentialBackoff(20)
		// maxDelay is 120s +/- 20% jitter -> [96s, 144s]
		assert.GreaterOrEqual(t, d, time.Duration(90*time.Second))
		assert.LessOrEqual(t, d, time.Duration(150*time.Second))
	})
}

func TestCustomDNSResolver_RateLimiting_MinResolutionInterval(t *testing.T) {
	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	origTimeNow := timeNowFunc
	timeNowFunc = func() time.Time { return baseTime }
	defer func() { timeNowFunc = origTimeNow }()

	timeUntilCh := make(chan time.Time, 1)
	origTimeUntil := timeUntilFunc
	timeUntilFunc = func(target time.Time) time.Duration {
		select {
		case timeUntilCh <- target:
		default:
		}
		return 0
	}
	defer func() { timeUntilFunc = origTimeUntil }()

	mockNet := newMockNetResolver()
	mockNet.hostResponse["rate-limit-host"] = []string{"192.0.2.1"}

	origNewNetResolver := newNetResolver
	newNetResolver = func(string) (netResolver, error) {
		return mockNet, nil
	}
	defer func() { newNetResolver = origNewNetResolver }()

	builder := NewDNSBuilder()
	cc := newMockClientConn()

	target := resolver.Target{
		URL: url.URL{Path: "/rate-limit-host:8080"},
	}
	r, err := builder.Build(target, cc, resolver.BuildOptions{})
	require.NoError(t, err)
	defer r.Close()

	// Wait for first resolution
	select {
	case s := <-cc.stateChan:
		require.Len(t, s.Addresses, 1)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first resolution")
	}

	// Request re-resolution via ResolveNow
	r.ResolveNow(resolver.ResolveNowOptions{})

	// Verify that the watcher waits until baseTime + MinResolutionInterval
	select {
	case targetTime := <-timeUntilCh:
		expectedTime := baseTime.Add(MinResolutionInterval)
		assert.Equal(t, expectedTime, targetTime)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for rate limit verification")
	}
}

func TestCustomDNSResolver_ErrorHandling_RetryBackoff(t *testing.T) {
	mockNet := newMockNetResolver()
	mockNet.hostError["retry-host"] = errors.New("initial dns failure")

	origNewNetResolver := newNetResolver
	newNetResolver = func(string) (netResolver, error) {
		return mockNet, nil
	}
	defer func() { newNetResolver = origNewNetResolver }()

	// Override timeUntilFunc to return 0 so backoff wait loops immediately in the test
	origTimeUntil := timeUntilFunc
	timeUntilFunc = func(time.Time) time.Duration {
		return 0
	}
	defer func() { timeUntilFunc = origTimeUntil }()

	builder := NewDNSBuilder()
	cc := newMockClientConn()

	target := resolver.Target{
		URL: url.URL{Path: "/retry-host:8080"},
	}
	r, err := builder.Build(target, cc, resolver.BuildOptions{})
	require.NoError(t, err)
	defer r.Close()

	// First resolution fails and reports error
	select {
	case err := <-cc.errChan:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "initial dns failure")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for initial error report")
	}

	// Clear error and provide valid IP to exercise retry recovery in watcher
	mockNet.mu.Lock()
	delete(mockNet.hostError, "retry-host")
	mockNet.hostResponse["retry-host"] = []string{"192.0.2.99"}
	mockNet.mu.Unlock()

	// Watcher retries via exponential backoff loop and succeeds
	select {
	case s := <-cc.stateChan:
		require.Len(t, s.Addresses, 1)
		assert.Equal(t, "192.0.2.99:8080", s.Addresses[0].Addr)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for recovered state update")
	}
}
