package test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// Cert is a certificate for tests. It was generated like this:
// 		opts := tls.CertOptions{Hosts: []string{"localhost"}, Organization: "Acme"}
//		certBytes, privKey, err := tls.generatePEM(opts)
var Cert = []byte(`-----BEGIN CERTIFICATE-----
MIIC8zCCAdugAwIBAgIQCSoocl6e/FR4mQy1wX6NbjANBgkqhkiG9w0BAQsFADAP
MQ0wCwYDVQQKEwRBY21lMB4XDTIyMDYyMjE3Mjk1MloXDTIzMDYyMjE3Mjk1Mlow
DzENMAsGA1UEChMEQWNtZTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
ANih5Kdn3tEXh6gLfQYplhHnNq8lmSMoPY7wdwXT95sxX9GzrVpR5tRQBExcR+Ie
Y2AElGmlhMETTchpU9RoU6fozjAuMYTkm+f0pyNnbdhCE5LnUBSrEhVHSQJ3ajs5
I6z9qS+H4uG+yVobiwzt+rnwD+Jdpt7ZwLHhkkkyHHFr8yxRVLN8LBzh8TnCRgj9
We64s8ZepkymC/2fhh6jdezibJQ3/dNbj17FHgwmC9oooBj4QwKOpPDzrH26aixu
6aAg0yudBS50uahKHI8bfieGYwRFk1PwzhV1mLLc324ZvDmT0KUkhIgQsaYPs47Z
EHwsmcVweUUPOAmO/H1ziPUCAwEAAaNLMEkwDgYDVR0PAQH/BAQDAgWgMBMGA1Ud
JQQMMAoGCCsGAQUFBwMBMAwGA1UdEwEB/wQCMAAwFAYDVR0RBA0wC4IJbG9jYWxo
b3N0MA0GCSqGSIb3DQEBCwUAA4IBAQA+8cGJfYRhXQxan7FATsbtC+1DwW1cPc60
5eLOuI0jPdvXLDmtOulBEjR4KOfJ5oTKXGjs/+gR3sffP6s8gm2XFQn4+OsmxHbO
b2RjPHgKUtJmrI4ZCN8iPGlKIar5u6Q8NZwzpeZ2XL0bpPp7RQsfHqMyhsqDinWR
vvwQB+Bri0oIOtzW2645vWmYc2SaFMd8+8g6Ipa+PRSJezeUxIVZG12zlhsio18F
9SHY2ONcYISjfrGTIcu4cZRGxCZGTIwMngBlb71mia+K7uH+UE6qfJy/t6KiFsCP
yOwMb95nGQSQLDNoGr8gwgE2qPuR0kR9Z5OrWF0DoVCyL3xnxr02
-----END CERTIFICATE-----`)

// PrivateKey is an RSA key used only for tests.
var PrivateKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEA2KHkp2fe0ReHqAt9BimWEec2ryWZIyg9jvB3BdP3mzFf0bOt
WlHm1FAETFxH4h5jYASUaaWEwRNNyGlT1GhTp+jOMC4xhOSb5/SnI2dt2EITkudQ
FKsSFUdJAndqOzkjrP2pL4fi4b7JWhuLDO36ufAP4l2m3tnAseGSSTIccWvzLFFU
s3wsHOHxOcJGCP1Z7rizxl6mTKYL/Z+GHqN17OJslDf901uPXsUeDCYL2iigGPhD
Ao6k8POsfbpqLG7poCDTK50FLnS5qEocjxt+J4ZjBEWTU/DOFXWYstzfbhm8OZPQ
pSSEiBCxpg+zjtkQfCyZxXB5RQ84CY78fXOI9QIDAQABAoIBAG8jL0FLIp62qZvm
uO9ualUo/37/lP7aaCpq50UQJ9lwjS3yNh8+IWQO4QWj2iUBXg4mi1Vf2ymKk78b
eixgkXp1D0Lcj/8ToYBwnUami04FKDGXhhf0Y8SS27vuM4vKlqjrQd7modkangYi
V0X82UKHDD8fuLpfkGIxzXDLypfMzjMuVpSntnWaf2YX3VR/0/66yEp9GejftF2k
wqhGoWM6r68pN5XuCqWd5PRluSoDy/o4BAFMhYCSfp9PjgZE8aoeWHgYzlZ3gUyn
r+HaDDNWbibhobXk/9h8lwAJ6KCZ5RZ+HFfh0HuwIxmocT9OCFgy/S0g1p+o3m9K
VNd5AMkCgYEA5fbS5UK7FBzuLoLgr1hktmbLJhpt8y8IPHNABHcUdE+O4/1xTQNf
pMUwkKjGG1MtrGjLOIoMGURKKn8lR1GMZueOTSKY0+mAWUGvSzl6vwtJwvJruT8M
otEO03o0tPnRKGxbFjqxkp2b6iqJ8MxCRZ3lSidc4mdi7PHzv9lwgvsCgYEA8Siq
7weCri9N6y+tIdORAXgRzcW54BmJyqB147c72RvbMacb6rN28KXpM3qnRXyp3Llb
yh81TW3FH10GqrjATws7BK8lP9kkAw0Z/7kNiS1NgH3pUbO+5H2kAa/6QW35nzRe
Jw2lyfYGWqYO4hYXH14ML1kjgS1hgd3XHOQ64M8CgYAKcjDYSzS2UC4dnMJaFLjW
dErsGy09a7iDDnUs/r/GHMsP3jZkWi/hCzgOiiwdl6SufUAl/FdaWnjH/2iRGco3
7nLPXC/3CFdVNp+g2iaSQRADtAFis9N+HeL/hkCYq/RtUqa8lsP0NgacF3yWnKCy
Ct8chDc67ZlXzBHXeCgdOwKBgHHGFPbWXUHeUW1+vbiyvrupsQSanznp8oclMtkv
Dk48hSokw9fzuU6Jh77gw9/Vk7HtxS9Tj+squZA1bDrJFPl1u+9WzkUUJZhG6xgp
bwhj1iejv5rrKUlVOTYOlwudXeJNa4oTNz9UEeVcaLMjZt9GmIsSC90a0uDZD26z
AlAjAoGAEoqm2DcNN7SrH6aVFzj1EVOrNsHYiXj/yefspeiEmf27PSAslP+uF820
SDpz4h+Bov5qTKkzcxuu1QWtA4M0K8Iy6IYLwb83DZEm1OsAf4i0pODz21PY/I+O
VHzjB10oYgaInHZgMUdyb6F571UdiYSB6a/IlZ3ngj5touy3VIM=
-----END RSA PRIVATE KEY-----`)

func dexMockHandler(t *testing.T, url string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/api/dex/.well-known/openid-configuration":
			_, err := io.WriteString(w, fmt.Sprintf(`
{
  "issuer": "%[1]s/api/dex",
  "authorization_endpoint": "%[1]s/api/dex/auth",
  "token_endpoint": "%[1]s/api/dex/token",
  "jwks_uri": "%[1]s/api/dex/keys",
  "userinfo_endpoint": "%[1]s/api/dex/userinfo",
  "device_authorization_endpoint": "%[1]s/api/dex/device/code",
  "grant_types_supported": ["authorization_code"],
  "response_types_supported": ["code"],
  "subject_types_supported": ["public"],
  "id_token_signing_alg_values_supported": ["RS512"],
  "code_challenge_methods_supported": ["S256", "plain"],
  "scopes_supported": ["openid"],
  "token_endpoint_auth_methods_supported": ["client_secret_basic", "client_secret_post"],
  "claims_supported": ["sub", "aud", "exp"]
}`, url))
			require.NoError(t, err)
		default:
			w.WriteHeader(404)
		}
	}
}

func GetDexTestServer(t *testing.T) *httptest.Server {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Start with a placeholder. We need the server URL before setting up the real handler.
	}))
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dexMockHandler(t, ts.URL)(w, r)
	})
	return ts
}

func oidcMockHandler(t *testing.T, url string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/.well-known/openid-configuration":
			_, err := io.WriteString(w, fmt.Sprintf(`
{
  "issuer": "%[1]s",
  "authorization_endpoint": "%[1]s/auth",
  "token_endpoint": "%[1]s/token",
  "jwks_uri": "%[1]s/keys",
  "userinfo_endpoint": "%[1]s/userinfo",
  "device_authorization_endpoint": "%[1]s/device/code",
  "grant_types_supported": ["authorization_code"],
  "response_types_supported": ["code"],
  "subject_types_supported": ["public"],
  "id_token_signing_alg_values_supported": ["RS512"],
  "code_challenge_methods_supported": ["S256", "plain"],
  "scopes_supported": ["openid"],
  "token_endpoint_auth_methods_supported": ["client_secret_basic", "client_secret_post"],
  "claims_supported": ["sub", "aud", "exp"]
}`, url))
			require.NoError(t, err)
		default:
			w.WriteHeader(404)
		}
	}
}

func GetOIDCTestServer(t *testing.T) *httptest.Server {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Start with a placeholder. We need the server URL before setting up the real handler.
	}))
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		oidcMockHandler(t, ts.URL)(w, r)
	})
	return ts
}