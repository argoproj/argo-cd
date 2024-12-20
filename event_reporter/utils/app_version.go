package utils

import (
	"encoding/json"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/events"
	"github.com/argoproj/argo-cd/v2/pkg/sources_server_client"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
)

func RepoAppVersionsToEvent(applicationVersions *apiclient.ApplicationVersions) (*events.ApplicationVersions, error) {
	applicationVersionsEvents := &events.ApplicationVersions{}
	applicationVersionsData, _ := json.Marshal(applicationVersions)
	err := json.Unmarshal(applicationVersionsData, applicationVersionsEvents)
	if err != nil {
		return nil, err
	}
	return applicationVersionsEvents, nil
}

func SourcesAppVersionsToRepo(applicationVersions *sources_server_client.AppVersionResult, logCtx *log.Entry) *apiclient.ApplicationVersions {
	if applicationVersions == nil {
		return nil
	}
	applicationVersionsRepo := &apiclient.ApplicationVersions{}
	applicationVersionsData, _ := json.Marshal(applicationVersions)
	err := json.Unmarshal(applicationVersionsData, applicationVersionsRepo)
	if err != nil {
		logCtx.Errorf("can't unmarshal app version: %v", err)
		return nil
	}
	return applicationVersionsRepo
}
