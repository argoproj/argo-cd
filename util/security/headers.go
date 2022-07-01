package security

import "net/http"

type Headers struct {
	XFrameOptions string
	ContentSecurityPolicy string
}

func SetSecurityHeaders(w http.ResponseWriter, headers Headers) {
	if headers.XFrameOptions != "" {
		w.Header().Set("X-Frame-Options", headers.XFrameOptions)
	}
	if headers.ContentSecurityPolicy != "" {
		w.Header().Set("Content-Security-Policy", headers.ContentSecurityPolicy)
	}
	w.Header().Set("X-XSS-Protection", "1")
}