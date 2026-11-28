package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

// SSHProxyURLEnv is the environment variable through which SSHCreds passes the SOCKS5 proxy URL,
// credentials included, to the argocd-ssh-proxy command that ssh runs as its ProxyCommand. Passing
// it in the environment keeps the URL out of the shell-interpreted GIT_SSH_COMMAND and out of the
// process list.
const SSHProxyURLEnv = "ARGOCD_SSH_PROXY_URL"

// sshProxyDialTimeout bounds connecting to the proxy and the proxy connecting to the Git server.
const sshProxyDialTimeout = 30 * time.Second

// ProxySSHConnection connects to host:port through the SOCKS5 proxy at proxyURL and relays that
// connection over in and out until the remote side closes it or reading from in fails. It
// implements the argocd-ssh-proxy command, which ssh runs as its ProxyCommand (with in and out
// being ssh's end of the connection) for SSH repositories that have a proxy configured.
//
// Host names are resolved by the proxy for both the socks5 and socks5h schemes, as connect-proxy,
// which Argo CD used before, does by default.
func ProxySSHConnection(ctx context.Context, proxyURL, host, port string, in io.Reader, out io.Writer) error {
	if err := validateProxyURL(proxyURL); err != nil {
		return fmt.Errorf("unsafe proxy URL rejected: %w", err)
	}
	parsedProxyURL, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("could not parse proxy URL: %w", err)
	}
	dialer, err := proxy.FromURL(parsedProxyURL, &net.Dialer{Timeout: sshProxyDialTimeout})
	if err != nil {
		return fmt.Errorf("could not create a dialer for proxy %s: %w", parsedProxyURL.Redacted(), err)
	}
	contextDialer, ok := dialer.(proxy.ContextDialer)
	if !ok {
		return fmt.Errorf("the dialer for proxy %s does not support contexts", parsedProxyURL.Redacted())
	}

	target := net.JoinHostPort(host, port)
	dialCtx, cancel := context.WithTimeout(ctx, sshProxyDialTimeout)
	defer cancel()
	conn, err := contextDialer.DialContext(dialCtx, "tcp", target)
	if err != nil {
		return fmt.Errorf("could not connect to %s through proxy %s: %w", target, parsedProxyURL.Redacted(), err)
	}
	defer conn.Close()

	// The connection ends when the remote side closes it. ssh closing its end (EOF on in) only means
	// it has nothing more to send, so keep relaying what the server still sends; ssh stops the proxy
	// command once it is done.
	inErr := make(chan error, 1)
	outErr := make(chan error, 1)
	go func() {
		_, err := io.Copy(conn, in)
		inErr <- err
	}()
	go func() {
		_, err := io.Copy(out, conn)
		outErr <- err
	}()

	for {
		select {
		case err := <-outErr:
			if err != nil && !errors.Is(err, net.ErrClosed) {
				return fmt.Errorf("relaying from %s: %w", target, err)
			}
			return nil
		case err := <-inErr:
			if err != nil {
				return fmt.Errorf("relaying to %s: %w", target, err)
			}
			inErr = nil // EOF from ssh: wait for the remote side to finish
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
