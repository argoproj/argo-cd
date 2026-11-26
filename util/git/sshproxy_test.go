package git

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// socksRequest is the target of a CONNECT request a socks5TestServer received.
type socksRequest struct {
	// domainName is true when the client sent the target as a host name (ATYP 3), leaving the
	// resolution to the proxy, and false when it sent an IP address.
	domainName bool
	host       string
	port       int
}

// socks5TestServer is a minimal SOCKS5 proxy (RFC 1928, with RFC 1929 username/password
// authentication when user is set) that supports CONNECT and records each request.
type socks5TestServer struct {
	addr       string
	user, pass string
	requests   chan socksRequest
}

// listenTCP listens on a free port on the loopback interface for the duration of the test.
func listenTCP(t *testing.T) net.Listener {
	t.Helper()
	l, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	return l
}

func startSOCKS5TestServer(t *testing.T, user, pass string) *socks5TestServer {
	t.Helper()
	l := listenTCP(t)
	t.Cleanup(func() { _ = l.Close() })
	s := &socks5TestServer{addr: l.Addr().String(), user: user, pass: pass, requests: make(chan socksRequest, 10)}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go s.serve(t.Context(), conn)
		}
	}()
	return s
}

func (s *socks5TestServer) serve(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	readN := func(n int) ([]byte, error) {
		b := make([]byte, n)
		_, err := io.ReadFull(r, b)
		return b, err
	}

	// Method negotiation.
	head, err := readN(2)
	if err != nil || head[0] != 5 {
		return
	}
	methods, err := readN(int(head[1]))
	if err != nil {
		return
	}
	want := byte(0) // no authentication
	if s.user != "" {
		want = 2 // username/password
	}
	if !bytes.Contains(methods, []byte{want}) {
		_, _ = conn.Write([]byte{5, 0xff})
		return
	}
	if _, err := conn.Write([]byte{5, want}); err != nil {
		return
	}
	if s.user != "" {
		// RFC 1929: VER ULEN UNAME PLEN PASSWD
		b, err := readN(2)
		if err != nil {
			return
		}
		user, err := readN(int(b[1]))
		if err != nil {
			return
		}
		plen, err := readN(1)
		if err != nil {
			return
		}
		pass, err := readN(int(plen[0]))
		if err != nil {
			return
		}
		if string(user) != s.user || string(pass) != s.pass {
			_, _ = conn.Write([]byte{1, 1})
			return
		}
		if _, err := conn.Write([]byte{1, 0}); err != nil {
			return
		}
	}

	// Request: VER CMD RSV ATYP DST.ADDR DST.PORT
	req, err := readN(4)
	if err != nil || req[1] != 1 {
		return
	}
	var parsed socksRequest
	switch req[3] {
	case 1:
		ip, err := readN(4)
		if err != nil {
			return
		}
		parsed.host = net.IP(ip).String()
	case 3:
		l, err := readN(1)
		if err != nil {
			return
		}
		name, err := readN(int(l[0]))
		if err != nil {
			return
		}
		parsed.domainName, parsed.host = true, string(name)
	case 4:
		ip, err := readN(16)
		if err != nil {
			return
		}
		parsed.host = net.IP(ip).String()
	default:
		return
	}
	port, err := readN(2)
	if err != nil {
		return
	}
	parsed.port = int(binary.BigEndian.Uint16(port))
	s.requests <- parsed

	target, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(parsed.host, strconv.Itoa(parsed.port)))
	if err != nil {
		_, _ = conn.Write([]byte{5, 5, 0, 1, 0, 0, 0, 0, 0, 0}) // connection refused
		return
	}
	defer target.Close()
	if _, err := conn.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}
	go func() { _, _ = io.Copy(target, r) }()
	_, _ = io.Copy(conn, target)
}

// startLineEchoServer starts a TCP server that reads one line, writes it back prefixed with
// "echo: " and closes the connection.
func startLineEchoServer(t *testing.T) (host, port string) {
	t.Helper()
	l := listenTCP(t)
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				line, err := bufio.NewReader(conn).ReadString('\n')
				if err != nil {
					return
				}
				_, _ = fmt.Fprintf(conn, "echo: %s", line)
			}()
		}
	}()
	var err error
	host, port, err = net.SplitHostPort(l.Addr().String())
	require.NoError(t, err)
	return host, port
}

func TestProxySSHConnection_RelaysThroughProxy(t *testing.T) {
	_, port := startLineEchoServer(t)

	for _, tc := range []struct {
		name       string
		scheme     string
		host       string
		domainName bool
	}{
		{"socks5 sends host names to the proxy", "socks5", "localhost", true},
		{"socks5h sends host names to the proxy", "socks5h", "localhost", true},
		{"IP addresses are sent as addresses", "socks5", "127.0.0.1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proxy := startSOCKS5TestServer(t, "", "")
			var out bytes.Buffer

			err := ProxySSHConnection(t.Context(), tc.scheme+"://"+proxy.addr, tc.host, port, strings.NewReader("hello\n"), &out)
			require.NoError(t, err)
			assert.Equal(t, "echo: hello\n", out.String())

			req := <-proxy.requests
			assert.Equal(t, tc.domainName, req.domainName)
			assert.Equal(t, tc.host, req.host)
			assert.Equal(t, port, strconv.Itoa(req.port))
		})
	}
}

func TestProxySSHConnection_UsernamePassword(t *testing.T) {
	host, port := startLineEchoServer(t)
	proxy := startSOCKS5TestServer(t, "user", "p@ss:word")

	t.Run("valid credentials", func(t *testing.T) {
		var out bytes.Buffer
		err := ProxySSHConnection(t.Context(), "socks5://user:p%40ss%3Aword@"+proxy.addr, host, port, strings.NewReader("hello\n"), &out)
		require.NoError(t, err)
		assert.Equal(t, "echo: hello\n", out.String())
	})

	t.Run("wrong credentials", func(t *testing.T) {
		var out bytes.Buffer
		err := ProxySSHConnection(t.Context(), "socks5://user:wrong@"+proxy.addr, host, port, strings.NewReader("hello\n"), &out)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "wrong", "the proxy password must not appear in errors")
		assert.Empty(t, out.String())
	})
}

func TestProxySSHConnection_KeepsRelayingAfterInputEOF(t *testing.T) {
	// The server writes a banner and closes without reading anything, like an SSH server ending a
	// connection: what it sends must arrive even though there is no input at all.
	l := listenTCP(t)
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		time.Sleep(100 * time.Millisecond) // after the client has seen EOF on its input
		_, _ = conn.Write([]byte("SSH-2.0-test\r\n"))
		_ = conn.Close()
	}()
	host, port, err := net.SplitHostPort(l.Addr().String())
	require.NoError(t, err)
	proxy := startSOCKS5TestServer(t, "", "")

	var out bytes.Buffer
	err = ProxySSHConnection(t.Context(), "socks5://"+proxy.addr, host, port, strings.NewReader(""), &out)
	require.NoError(t, err)
	assert.Equal(t, "SSH-2.0-test\r\n", out.String())
}

func TestProxySSHConnection_Errors(t *testing.T) {
	t.Run("unsafe proxy URL", func(t *testing.T) {
		err := ProxySSHConnection(t.Context(), "socks5://evil'$(id)'host:1080", "git.example.com", "22", strings.NewReader(""), io.Discard)
		require.ErrorContains(t, err, "unsafe proxy URL rejected")
	})

	t.Run("malformed proxy URL", func(t *testing.T) {
		err := ProxySSHConnection(t.Context(), "socks5://user:secret@127.0.0.1:bad", "git.example.com", "22", strings.NewReader(""), io.Discard)
		require.ErrorContains(t, err, "unsafe proxy URL rejected")
		assert.NotContains(t, err.Error(), "secret", "the proxy password must not appear in errors")
	})

	t.Run("unsupported scheme", func(t *testing.T) {
		err := ProxySSHConnection(t.Context(), "http://127.0.0.1:1080", "git.example.com", "22", strings.NewReader(""), io.Discard)
		require.ErrorContains(t, err, "unsafe proxy URL rejected")
	})

	t.Run("proxy unreachable", func(t *testing.T) {
		l := listenTCP(t)
		addr := l.Addr().String()
		require.NoError(t, l.Close())

		err := ProxySSHConnection(t.Context(), "socks5://user:secret@"+addr, "git.example.com", "22", strings.NewReader(""), io.Discard)
		require.ErrorContains(t, err, "could not connect to git.example.com:22 through proxy")
		assert.NotContains(t, err.Error(), "secret", "the proxy password must not appear in errors")
	})

	t.Run("target unreachable", func(t *testing.T) {
		l := listenTCP(t)
		host, port, err := net.SplitHostPort(l.Addr().String())
		require.NoError(t, err)
		require.NoError(t, l.Close())
		proxy := startSOCKS5TestServer(t, "", "")

		err = ProxySSHConnection(t.Context(), "socks5://"+proxy.addr, host, port, strings.NewReader(""), io.Discard)
		require.ErrorContains(t, err, "could not connect to "+net.JoinHostPort(host, port))
	})
}
