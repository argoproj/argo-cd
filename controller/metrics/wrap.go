package metrics

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"path"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
)

type K8sRequestVerb string

const (
	List    K8sRequestVerb = "List"
	Get     K8sRequestVerb = "Get"
	Create  K8sRequestVerb = "Create"
	Update  K8sRequestVerb = "Update"
	Patch   K8sRequestVerb = "Patch"
	Delete  K8sRequestVerb = "Delete"
	Unknown K8sRequestVerb = "Unknown"
)

const findPathRegex = `/v\d\w*?(/[a-zA-Z0-9-]*)(/[a-zA-Z0-9-]*)?(/[a-zA-Z0-9-]*)?(/[a-zA-Z0-9-]*)?`

var (
	processPath       = regexp.MustCompile(findPathRegex)
	isNamespacedQuery = regexp.MustCompile(`/.*/namespaces/[a-z0-9-]+/[a-z0-9-]+(/[a-z0-9-]+)?`)
)

type ResourceInfo struct {
	Kind       string
	Namespace  string
	Name       string
	Verb       K8sRequestVerb
	StatusCode int
}

func (ri ResourceInfo) HasAllFields() bool {
	return ri.Kind != "" && ri.Namespace != "" && ri.Name != "" && ri.Verb != "" && ri.StatusCode != 0
}

type metricsRoundTripper struct {
	roundTripper http.RoundTripper
	inc          func(ResourceInfo) error
}

// isGetOrList Uses a path from a request to determine if the request is a GET or LIST. The function tries to find an
// API version within the path and then calculates how many remaining segments are after the API version. A LIST request
// has segments for the kind with a namespace and the specific namespace if the kind is a namespaced resource.
// Meanwhile a GET request has an additional segment for resource name. As a result, a LIST has an odd number of
// segments while a GET request has an even number of segments.
func isGetOrList(r *http.Request) K8sRequestVerb {
	segments := processPath.FindStringSubmatch(r.URL.Path)
	unusedGroup := 0
	for _, str := range segments {
		if str == "" {
			unusedGroup++
		}
	}
	if unusedGroup%2 == 1 {
		return List
	}
	return Get
}

func resolveK8sRequestVerb(r *http.Request) K8sRequestVerb {
	if r.Method == "POST" {
		return Create
	}
	if r.Method == "DELETE" {
		return Delete
	}
	if r.Method == "PATCH" {
		return Patch
	}
	if r.Method == "PUT" {
		return Update
	}
	if r.Method == "GET" {
		return isGetOrList(r)
	}
	return Unknown
}

func handleCreate(r *http.Request) ResourceInfo {
	kind := path.Base(r.URL.Path)
	bodyIO, err := r.GetBody()
	if err != nil {
		log.WithField("Kind", kind).Warnf("Unable to Process Create request: %v", err)
		return ResourceInfo{}
	}
	body, err := ioutil.ReadAll(bodyIO)
	if err != nil {
		log.WithField("Kind", kind).Warnf("Unable to Process Create request: %v", err)
		return ResourceInfo{}
	}
	var obj map[string]interface{}
	err = json.Unmarshal(body, &obj)
	if err != nil {
		log.WithField("Kind", kind).Warnf("Unable to Process Create request: %v", err)
		return ResourceInfo{}
	}
	un := unstructured.Unstructured{Object: obj}
	return ResourceInfo{
		Kind:      kind,
		Namespace: un.GetNamespace(),
		Name:      un.GetName(),
		Verb:      Create,
	}
}

func (mrt *metricsRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, roundTimeErr := mrt.roundTripper.RoundTrip(r)
	info := parseRequest(r)
	info.StatusCode = resp.StatusCode
	mrt.inc(info)
	return resp, roundTimeErr
}

func parseRequest(r *http.Request) ResourceInfo {
	var info ResourceInfo
	verb := resolveK8sRequestVerb(r)
	path := strings.Split(r.URL.Path, "/")
	len := len(path)
	switch verb {
	case List:
		info.Kind = path[len-1]
		if isNamespacedQuery.MatchString(r.URL.Path) {
			info.Namespace = path[len-2]
		}
	case Create:
		info = handleCreate(r)
	case Get, Delete, Patch, Update:
		info.Name = path[len-1]
		info.Kind = path[len-2]
		if isNamespacedQuery.MatchString(r.URL.Path) {
			info.Namespace = path[len-3]
		}
	default:
		log.WithField("path", r.URL.Path).WithField("method", r.Method).Warnf("Unknown Request")
	}
	info.Verb = verb
	return info
}

// AddMetricsTransportWrapper adds a transport wrapper which wraps a function call around each kubernetes request
func AddMetricsTransportWrapper(config *rest.Config, incFunc func(ResourceInfo) error) *rest.Config {
	wrap := config.WrapTransport
	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		if wrap != nil {
			rt = wrap(rt)
		}
		return &metricsRoundTripper{
			roundTripper: rt,
			inc:          incFunc,
		}
	}
	return config
}
