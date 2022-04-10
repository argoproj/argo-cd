package apiclient

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	argocderrors "github.com/argoproj/argo-cd/v2/util/errors"
	argoio "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/rand"
)

const (
	frameHeaderLength = 5
	endOfStreamFlag   = 128
)

type noopCodec struct{}

func (noopCodec) Marshal(v interface{}) ([]byte, error) {
	return v.([]byte), nil
}

func (noopCodec) Unmarshal(data []byte, v interface{}) error {
	pointer := v.(*[]byte)
	*pointer = data
	return nil
}

func (noopCodec) String() string {
	return "bytes"
}

func toFrame(msg []byte) []byte {
	frame := append([]byte{0, 0, 0, 0}, msg...)
	binary.BigEndian.PutUint32(frame, uint32(len(msg)))
	frame = append([]byte{0}, frame...)
	return frame
}

func (c *client) executeRequest(fullMethodName string, msg []byte, md metadata.MD) (*http.Response, error) {
	schema := "https"
	if c.PlainText {
		schema = "http"
	}
	rootPath := strings.TrimRight(strings.TrimLeft(c.GRPCWebRootPath, "/"), "/")

	var requestURL string
	if rootPath != "" {
		requestURL = fmt.Sprintf("%s://%s/%s%s", schema, c.ServerAddr, rootPath, fullMethodName)
	} else {
		requestURL = fmt.Sprintf("%s://%s%s", schema, c.ServerAddr, fullMethodName)
	}
	req, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewReader(toFrame(msg)))

	if err != nil {
		return nil, err
	}
	for k, v := range md {
		if strings.HasPrefix(k, ":") {
			continue
		}
		for i := range v {
			req.Header.Set(k, v[i])
		}
	}
	req.Header.Set("content-type", "application/grpc-web+proto")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s %s failed with status code %d", req.Method, req.URL, resp.StatusCode)
	}
	var code codes.Code
	if statusStr := resp.Header.Get("Grpc-Status"); statusStr != "" {
		statusInt, err := strconv.ParseUint(statusStr, 10, 32)
		if err != nil {
			code = codes.Unknown
		} else {
			code = codes.Code(statusInt)
		}
		if code != codes.OK {
			return nil, status.Error(code, resp.Header.Get("Grpc-Message"))
		}
	}
	return resp, nil
}

func (c *client) startGRPCProxy() (*grpc.Server, net.Listener, error) {
	serverAddr := fmt.Sprintf("%s/argocd-%s.sock", os.TempDir(), rand.RandString(16))
	ln, err := net.Listen("unix", serverAddr)

	if err != nil {
		return nil, nil, err
	}
	proxySrv := grpc.NewServer(
		grpc.CustomCodec(&noopCodec{}),
		grpc.UnknownServiceHandler(func(srv interface{}, stream grpc.ServerStream) error {
			fullMethodName, ok := grpc.MethodFromServerStream(stream)
			if !ok {
				return fmt.Errorf("Unable to get method name from stream context.")
			}
			msg := make([]byte, 0)
			err := stream.RecvMsg(&msg)
			if err != nil {
				return err
			}

			md, _ := metadata.FromIncomingContext(stream.Context())

			for _, kv := range c.Headers {
				if len(strings.Split(kv, ":"))%2 == 1 {
					return fmt.Errorf("additional headers key/values must be separated by a colon(:): %s", kv)
				}
				md.Append(strings.Split(kv, ":")[0], strings.Split(kv, ":")[1])
			}

			resp, err := c.executeRequest(fullMethodName, msg, md)
			if err != nil {
				return err
			}

			go func() {
				<-stream.Context().Done()
				argoio.Close(resp.Body)
			}()
			defer argoio.Close(resp.Body)
			c.httpClient.CloseIdleConnections()

			for {
				header := make([]byte, frameHeaderLength)
				if _, err := io.ReadAtLeast(resp.Body, header, frameHeaderLength); err != nil {
					if err == io.EOF {
						err = io.ErrUnexpectedEOF
					}
					return err
				}

				if header[0] == endOfStreamFlag {
					return nil
				}
				length := int(binary.BigEndian.Uint32(header[1:frameHeaderLength]))
				data := make([]byte, length)

				if read, err := io.ReadAtLeast(resp.Body, data, length); err != nil {
					if err != io.EOF {
						return err
					} else if read < length {
						return io.ErrUnexpectedEOF
					} else {
						return nil
					}
				}

				if err := stream.SendMsg(data); err != nil {
					return err
				}

			}
		}))
	go func() {
		err := proxySrv.Serve(ln)
		argocderrors.CheckError(err)
	}()
	return proxySrv, ln, nil
}

// useGRPCProxy ensures that grpc proxy server is started and return closer which stops server when no one uses it
func (c *client) useGRPCProxy() (net.Addr, io.Closer, error) {
	c.proxyMutex.Lock()
	defer c.proxyMutex.Unlock()

	if c.proxyListener == nil {
		var err error
		c.proxyServer, c.proxyListener, err = c.startGRPCProxy()
		if err != nil {
			return nil, nil, err
		}
	}
	c.proxyUsersCount = c.proxyUsersCount + 1

	return c.proxyListener.Addr(), argoio.NewCloser(func() error {
		c.proxyMutex.Lock()
		defer c.proxyMutex.Unlock()
		c.proxyUsersCount = c.proxyUsersCount - 1
		if c.proxyUsersCount == 0 {
			c.proxyServer.Stop()
			c.proxyListener = nil
			c.proxyServer = nil
			return nil
		}
		return nil
	}), nil
}
