package utils

import (
	"encoding/json"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/events"
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
