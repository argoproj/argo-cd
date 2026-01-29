package conversion

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1beta1"
)

// Handler handles CRD conversion webhook requests for Application resources.
// It converts between v1alpha1 and v1beta1 API versions.
type Handler struct{}

// NewHandler creates a new conversion webhook handler.
func NewHandler() *Handler {
	return &Handler{}
}

// ServeHTTP handles the conversion webhook request.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "only POST is supported", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.WithError(err).Error("Failed to read conversion webhook request body")
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var review apiextensionsv1.ConversionReview
	if err := json.Unmarshal(body, &review); err != nil {
		log.WithError(err).Error("Failed to unmarshal ConversionReview")
		http.Error(w, "failed to parse ConversionReview", http.StatusBadRequest)
		return
	}

	if review.Request == nil {
		log.Error("ConversionReview request is nil")
		http.Error(w, "ConversionReview request is nil", http.StatusBadRequest)
		return
	}

	response := h.convert(review.Request)
	review.Response = response

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(review); err != nil {
		log.WithError(err).Error("Failed to encode ConversionReview response")
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) convert(req *apiextensionsv1.ConversionRequest) *apiextensionsv1.ConversionResponse {
	resp := &apiextensionsv1.ConversionResponse{
		UID: req.UID,
	}

	convertedObjects := make([]runtime.RawExtension, 0, len(req.Objects))

	for i, obj := range req.Objects {
		converted, err := h.convertObject(obj.Raw, req.DesiredAPIVersion)
		if err != nil {
			log.WithError(err).WithField("index", i).Error("Failed to convert object")
			resp.Result = metav1.Status{
				Status:  metav1.StatusFailure,
				Message: fmt.Sprintf("failed to convert object at index %d: %v", i, err),
			}
			return resp
		}
		convertedObjects = append(convertedObjects, runtime.RawExtension{Raw: converted})
	}

	resp.Result = metav1.Status{Status: metav1.StatusSuccess}
	resp.ConvertedObjects = convertedObjects
	return resp
}

func (h *Handler) convertObject(raw []byte, desiredVersion string) ([]byte, error) {
	// First, determine the current API version by partial unmarshal
	var meta struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, fmt.Errorf("failed to extract apiVersion/kind: %w", err)
	}

	// Only handle Application conversions
	if meta.Kind != "Application" {
		return nil, fmt.Errorf("unsupported kind: %s (only Application is supported)", meta.Kind)
	}

	currentVersion := meta.APIVersion

	// Same version, no conversion needed
	if currentVersion == desiredVersion {
		return raw, nil
	}

	switch {
	case currentVersion == v1alpha1.SchemeGroupVersion.String() && desiredVersion == v1beta1.SchemeGroupVersion.String():
		return h.convertV1alpha1ToV1beta1(raw)

	case currentVersion == v1beta1.SchemeGroupVersion.String() && desiredVersion == v1alpha1.SchemeGroupVersion.String():
		return h.convertV1beta1ToV1alpha1(raw)

	default:
		return nil, fmt.Errorf("unsupported conversion: %s -> %s", currentVersion, desiredVersion)
	}
}

func (h *Handler) convertV1alpha1ToV1beta1(raw []byte) ([]byte, error) {
	var src v1alpha1.Application
	if err := json.Unmarshal(raw, &src); err != nil {
		return nil, fmt.Errorf("failed to unmarshal v1alpha1.Application: %w", err)
	}

	dst := v1beta1.ConvertFromV1alpha1(&src)

	result, err := json.Marshal(dst)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal v1beta1.Application: %w", err)
	}
	return result, nil
}

func (h *Handler) convertV1beta1ToV1alpha1(raw []byte) ([]byte, error) {
	var src v1beta1.Application
	if err := json.Unmarshal(raw, &src); err != nil {
		return nil, fmt.Errorf("failed to unmarshal v1beta1.Application: %w", err)
	}

	dst := v1beta1.ConvertToV1alpha1(&src)

	result, err := json.Marshal(dst)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal v1alpha1.Application: %w", err)
	}
	return result, nil
}
