package kubectl

import (
	"net/url"
	"regexp"
)

// The functions here are adapted from: https://github.com/argoproj/pkg/blob/f5a0a066030558f089fa645dc6546ddc5917bad5/kubeclientmetrics/metric.go

const findPathRegex = `/v\d\w*?(/[a-zA-Z0-9-]*)(/[a-zA-Z0-9-]*)?(/[a-zA-Z0-9-]*)?(/[a-zA-Z0-9-]*)?`

var processPath = regexp.MustCompile(findPathRegex)

// discernGetRequest uses a path from a request to determine if the request is a GET, LIST, or WATCH.
// The function tries to find an API version within the path and then calculates how many remaining
// segments are after the API version. A LIST/WATCH request has segments for the kind with a
// namespace and the specific namespace if the kind is a namespaced resource. Meanwhile a GET
// request has an additional segment for resource name. As a result, a LIST/WATCH has an odd number
// of segments while a GET request has an even number of segments. Watch is determined if the query
// parameter watch=true is present in the request.
func discernGetRequest(u url.URL) string {
	segments := processPath.FindStringSubmatch(u.Path)
	unusedGroup := 0
	for _, str := range segments {
		if str == "" {
			unusedGroup++
		}
	}
	if unusedGroup%2 == 1 {
		if watchQueryParamValues, ok := u.Query()["watch"]; ok {
			if len(watchQueryParamValues) > 0 && watchQueryParamValues[0] == "true" {
				return "Watch"
			}
		}
		return "List"
	}
	return "Get"
}

func resolveK8sRequestVerb(u url.URL, method string) string {
	if method == "POST" {
		return "Create"
	}
	if method == "DELETE" {
		return "Delete"
	}
	if method == "PATCH" {
		return "Patch"
	}
	if method == "PUT" {
		return "Update"
	}
	if method == "GET" {
		return discernGetRequest(u)
	}
	return "Unknown"
}
