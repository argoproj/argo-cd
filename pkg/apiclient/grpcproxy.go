package apiclient

import (
	"bytes"
	"crypto/tls"
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

	argocderrors "github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/rand"
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
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s://%s%s", schema, c.ServerAddr, fullMethodName), bytes.NewReader(toFrame(msg)))
	if err != nil {
		return nil, err
	}
	if md != nil {
		for k, v := range md {
			if strings.HasPrefix(k, ":") {
				continue
			}
			for i := range v {
				req.Header.Set(k, v[i])
			}
		}
	}
	req.Header.Set("content-type", "application/grpc-web+proto")

	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: c.Insecure},
	}}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	var code codes.Code
	if statusStr := resp.Header.Get("Grpc-Status"); statusStr != "" {
		statusInt, err := strconv.Atoi(statusStr)
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
			err = stream.RecvMsg(&msg)
			if err != nil {
				return err
			}
			md, _ := metadata.FromIncomingContext(stream.Context())
			resp, err := c.executeRequest(fullMethodName, msg, md)
			if err != nil {
				return err
			}

			go func() {
				<-stream.Context().Done()
				util.Close(resp.Body)
			}()
			defer util.Close(resp.Body)

			for {
				header := make([]byte, frameHeaderLength)
				if _, err := resp.Body.Read(header); err != nil {
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

	return c.proxyListener.Addr(), util.NewCloser(func() error {
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
