package testdata

import _ "embed"

var (
	//go:embed ssa-revision-history/deployment.yaml
	SSARevisionHistoryDeployment string

	//go:embed guestbook/guestbook-ui-deployment.yaml
	GuestbookDeployment string
)
