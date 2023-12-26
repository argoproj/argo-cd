package handlers

import (
	"encoding/json"
	appclient "github.com/argoproj/argo-cd/v2/event_reporter/application"
	"github.com/argoproj/argo-cd/v2/event_reporter/sharding"
	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"net/http"
	"strconv"
	"strings"
)

type RequestHandlers struct {
	ApplicationServiceClient appclient.ApplicationClient
}

func GetRequestHandlers(applicationServiceClient appclient.ApplicationClient) *RequestHandlers {
	return &RequestHandlers{
		ApplicationServiceClient: applicationServiceClient,
	}
}

// queryParams: []string{"shardings"}
// response JSON { "strategyName": { Distribution, //Apps }
func (rH *RequestHandlers) GetAppDistribution(w http.ResponseWriter, r *http.Request) {
	type ShardingAlgorithmData struct {
		Distribution map[string]int `json:"distribution"`
		Apps         map[string]int `json:"apps"`
	}
	response := map[string]ShardingAlgorithmData{}

	shardings := []string{""}
	shardingsParam := r.URL.Query().Get("shardings")
	if shardingsParam != "" {
		shardings = strings.Split(shardingsParam, ",")
	}

	apps, err := rH.ApplicationServiceClient.List(r.Context(), &applicationpkg.ApplicationQuery{})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shardingInstance := sharding.NewSharding()

	for _, shardingAlgorithm := range shardings {
		distributionMap := make(map[string]int)
		appsMap := make(map[string]int)
		distributionFunction := shardingInstance.GetDistributionFunction(shardingAlgorithm)

		for _, app := range apps.Items {
			expectedShard := distributionFunction(&app)
			distributionMap[strconv.Itoa(expectedShard)] += 1
			appsMap[app.QualifiedName()] = expectedShard
		}

		shardingAlgorithmDisplayName := shardingAlgorithm
		if shardingAlgorithm == "" {
			shardingAlgorithmDisplayName = "default"
		}

		response[shardingAlgorithmDisplayName] = ShardingAlgorithmData{
			Distribution: distributionMap,
			Apps:         appsMap,
		}
	}

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(jsonBytes)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
