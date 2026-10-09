package grpc

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	grpcbackoff "google.golang.org/grpc/backoff"
	"google.golang.org/grpc/resolver"

	"github.com/argoproj/argo-cd/v3/util/env"
)

const (
	// EnvGRPCDisableCustomDNSResolver is the environment variable that, when set to
	// "true", disables Argo CD's custom DNS resolver and restores gRPC's
	// built-in default DNS resolver behavior.
	EnvGRPCDisableCustomDNSResolver = "ARGOCD_GRPC_DISABLE_CUSTOM_DNS_RESOLVER"

	defaultPort       = "443"
	defaultDNSSvrPort = "53"
)

var (
	// MinResolutionInterval is the minimum interval at which re-resolutions are
	// allowed. This prevents excessive DNS queries.
	MinResolutionInterval = 30 * time.Second

	// ResolvingTimeout specifies the maximum duration for a DNS resolution request.
	ResolvingTimeout = 30 * time.Second

	defaultDNSBuilder resolver.Builder

	errMissingAddr   = errors.New("dns: missing address")
	errEndsWithColon = errors.New("dns: error parsing address: missing port after colon")

	// timeNowFunc, timeAfterFunc, and timeUntilFunc allow deterministic time mocking in unit tests.
	timeNowFunc   = time.Now
	timeAfterFunc = time.After
	timeUntilFunc = time.Until
)

// netResolver defines the interface for DNS host lookups, allowing test mocks.
type netResolver interface {
	LookupHost(ctx context.Context, host string) (addrs []string, err error)
}

// newNetResolver creates a netResolver instance for the given authority (if specified).
var newNetResolver = func(authority string) (netResolver, error) {
	if authority == "" {
		return net.DefaultResolver, nil
	}

	host, port, err := parseTarget(authority, defaultDNSSvrPort)
	if err != nil {
		return nil, err
	}

	authorityWithPort := net.JoinHostPort(host, port)
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, network, authorityWithPort)
		},
	}, nil
}

func init() {
	// Capture gRPC's built-in default DNS resolver before registering our custom one.
	// This enables the kill-switch (EnvGRPCDisableCustomDNSResolver) to seamlessly
	// delegate to upstream behavior when toggled on.
	defaultDNSBuilder = resolver.Get("dns")
	resolver.Register(NewDNSBuilder())
}

// IsCustomDNSResolverDisabled returns whether the custom DNS resolver kill-switch is active.
func IsCustomDNSResolverDisabled() bool {
	return env.ParseBoolFromEnv(EnvGRPCDisableCustomDNSResolver, false)
}

// NewDNSBuilder creates a new dnsBuilder that produces custom DNS resolvers without SRV queries.
func NewDNSBuilder() resolver.Builder {
	return &customDNSBuilder{}
}

type customDNSBuilder struct{}

// Build creates and starts a custom DNS resolver that watches name resolution
// for the given target without issuing _grpclb._tcp SRV lookups.
func (b *customDNSBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	if IsCustomDNSResolverDisabled() && defaultDNSBuilder != nil {
		return defaultDNSBuilder.Build(target, cc, opts)
	}

	host, port, err := parseTarget(target.Endpoint(), defaultPort)
	if err != nil {
		return nil, err
	}

	// If host is a literal IP address (e.g. 127.0.0.1 or [::1]), no DNS resolution is needed.
	// If formatIP returns an error, host is a DNS hostname and we proceed to DNS resolution, matching upstream gRPC.
	if ipAddr, err := formatIP(host); err == nil {
		addr := []resolver.Address{{Addr: ipAddr + ":" + port}}
		if err := cc.UpdateState(resolver.State{
			Addresses: addr,
			Endpoints: []resolver.Endpoint{{Addresses: addr}},
		}); err != nil {
			return nil, err
		}
		return deadResolver{}, nil
	}

	// DNS address (hostname)
	ctx, cancel := context.WithCancel(context.Background())
	d := &customDNSResolver{
		host:   host,
		port:   port,
		ctx:    ctx,
		cancel: cancel,
		cc:     cc,
		rn:     make(chan struct{}, 1),
	}

	var auth string
	if target.URL.Host != "" {
		auth = target.URL.Host
	}
	d.resolver, err = newNetResolver(auth)
	if err != nil {
		cancel()
		return nil, err
	}

	d.wg.Add(1)
	go d.watcher()
	return d, nil
}

// Scheme returns "dns", replacing the default gRPC DNS scheme globally.
func (b *customDNSBuilder) Scheme() string {
	return "dns"
}

// deadResolver is a no-op resolver used for static IP literal targets.
type deadResolver struct{}

func (deadResolver) ResolveNow(resolver.ResolveNowOptions) {}
func (deadResolver) Close()                                {}

// customDNSResolver watches for hostname resolution updates without SRV queries.
type customDNSResolver struct {
	host     string
	port     string
	resolver netResolver
	ctx      context.Context
	cancel   context.CancelFunc
	cc       resolver.ClientConn
	rn       chan struct{}
	wg       sync.WaitGroup
}

// ResolveNow signals the resolver to re-resolve the target name.
// Re-resolution is rate-limited to occur no more frequently than MinResolutionInterval.
func (d *customDNSResolver) ResolveNow(resolver.ResolveNowOptions) {
	select {
	case d.rn <- struct{}{}:
	default:
	}
}

// Close terminates the resolver and waits for the watcher goroutine to exit.
func (d *customDNSResolver) Close() {
	d.cancel()
	d.wg.Wait()
}

func (d *customDNSResolver) watcher() {
	defer d.wg.Done()
	backoffIndex := 1
	for {
		state, err := d.lookup()
		if err != nil {
			d.cc.ReportError(err)
		} else {
			err = d.cc.UpdateState(*state)
		}

		var nextResolutionTime time.Time
		if err == nil {
			// Successful resolution. Wait for next ResolveNow() or at least MinResolutionInterval.
			backoffIndex = 1
			nextResolutionTime = timeNowFunc().Add(MinResolutionInterval)
			select {
			case <-d.ctx.Done():
				return
			case <-d.rn:
			}
		} else {
			// Exponential backoff upon error.
			backoffDuration := exponentialBackoff(backoffIndex)
			nextResolutionTime = timeNowFunc().Add(backoffDuration)
			backoffIndex++
		}

		select {
		case <-d.ctx.Done():
			return
		case <-timeAfterFunc(timeUntilFunc(nextResolutionTime)):
		}
	}
}

func (d *customDNSResolver) lookupHost(ctx context.Context) ([]resolver.Address, error) {
	addrs, err := d.resolver.LookupHost(ctx, d.host)
	if err != nil {
		err = handleDNSError(err)
		return nil, err
	}

	newAddrs := make([]resolver.Address, 0, len(addrs))
	for _, a := range addrs {
		ip, err := formatIP(a)
		if err != nil {
			return nil, fmt.Errorf("dns: error parsing IP address %v: %w", a, err)
		}
		addr := ip + ":" + d.port
		newAddrs = append(newAddrs, resolver.Address{Addr: addr})
	}
	return newAddrs, nil
}

func (d *customDNSResolver) lookup() (*resolver.State, error) {
	ctx, cancel := context.WithTimeout(d.ctx, ResolvingTimeout)
	defer cancel()

	// NOTE: Unlike upstream gRPC-Go's default DNS resolver when grpclb is linked,
	// we explicitly DO NOT perform any SRV queries (_grpclb._tcp.*), preventing
	// noisy DNS failures and NXDOMAIN lookups on Kubernetes clusters (issue #29989).
	addrs, hostErr := d.lookupHost(ctx)
	if hostErr != nil {
		return nil, hostErr
	}

	eps := make([]resolver.Endpoint, 0, len(addrs))
	for _, addr := range addrs {
		eps = append(eps, resolver.Endpoint{Addresses: []resolver.Address{addr}})
	}

	state := resolver.State{
		Addresses: addrs,
		Endpoints: eps,
	}
	return &state, nil
}

// formatIP formats an IPv4 address or brackets an IPv6 address.
func formatIP(addr string) (string, error) {
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return "", err
	}
	if ip.Is4() {
		return addr, nil
	}
	return "[" + addr + "]", nil
}

// parseTarget splits host and port from a target string.
func parseTarget(target, defaultPort string) (host, port string, err error) {
	if target == "" {
		return "", "", errMissingAddr
	}
	if _, err := netip.ParseAddr(target); err == nil {
		return target, defaultPort, nil
	}
	if host, port, err = net.SplitHostPort(target); err == nil {
		if port == "" {
			return "", "", errEndsWithColon
		}
		if host == "" {
			host = "localhost"
		}
		return host, port, nil
	}
	if host, port, err = net.SplitHostPort(target + ":" + defaultPort); err == nil {
		return host, port, nil
	}
	return "", "", fmt.Errorf("invalid target address %v, error info: %w", target, err)
}

func handleDNSError(err error) error {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && !dnsErr.IsTimeout && !dnsErr.IsTemporary {
		return nil
	}
	if err != nil {
		err = fmt.Errorf("dns: A record lookup error: %w", err)
		logrus.Debug(err)
	}
	return err
}

// exponentialBackoff calculates backoff duration with jitter, mirroring upstream gRPC DefaultExponential.
func exponentialBackoff(retries int) time.Duration {
	cfg := grpcbackoff.DefaultConfig
	if retries <= 0 {
		return cfg.BaseDelay
	}
	backoff, maxDelay := float64(cfg.BaseDelay), float64(cfg.MaxDelay)
	for backoff < maxDelay && retries > 0 {
		backoff *= cfg.Multiplier
		retries--
	}
	if backoff > maxDelay {
		backoff = maxDelay
	}

	// Randomize backoff delays so that if a cluster of requests start at
	// the same time, they won't operate in lockstep.
	backoff *= 1 + cfg.Jitter*(rand.Float64()*2-1)
	if backoff < 0 {
		return 0
	}
	return time.Duration(backoff)
}
