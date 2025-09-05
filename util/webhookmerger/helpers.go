package webhookmerger

import (
	"bytes"
	"io"
	"net/http"
)

func copyRequest(r *http.Request) (*http.Request, error) {
	// Clone context
	req := r.Clone(r.Context())

	// Clone the body, if needed
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		r.Body.Close()

		// Reset original body so it can still be used
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		// Set new body for the cloned request
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	return req, nil
}
