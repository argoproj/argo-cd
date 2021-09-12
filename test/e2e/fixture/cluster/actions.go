package cluster

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	clusterpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/cluster"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
)

// this implements the "when" part of given/when/then
//
// none of the func implement error checks, and that is complete intended, you should check for errors
// using the Then()
type Actions struct {
	context      *Context
	lastOutput   string
	lastError    error
	ignoreErrors bool
}

func (a *Actions) IgnoreErrors() *Actions {
	a.ignoreErrors = true
	return a
}

func (a *Actions) DoNotIgnoreErrors() *Actions {
	a.ignoreErrors = false
	return a
}

func (a *Actions) Create(args ...string) *Actions {
	_, clusterClient, _ := fixture.ArgoCDClientset.NewClusterClient()

	_, err := clusterClient.Create(context.Background(), &clusterpkg.ClusterCreateRequest{
		Cluster: &v1alpha1.Cluster{
			Server:             a.context.server,
			Name:               a.context.name,
			Config:             v1alpha1.ClusterConfig{},
			ConnectionState:    v1alpha1.ConnectionState{},
			ServerVersion:      "",
			Namespaces:         nil,
			RefreshRequestedAt: nil,
			Info:               v1alpha1.ClusterInfo{},
			Shard:              nil,
			ClusterResources:   false,
			Project:            a.context.project,
		},
		Upsert: a.context.upsert,
	})

	if err != nil {
		if !a.ignoreErrors {
			log.Fatalf(fmt.Sprintf("Failed to upsert cluster %v", err.Error()))
		}
		a.lastError = errors.New(err.Error())
	}

	return a
}

func (a *Actions) List() *Actions {
	a.context.t.Helper()
	a.runCli("cluster", "list")
	return a
}

func (a *Actions) Get() *Actions {
	a.context.t.Helper()
	a.runCli("cluster", "get", a.context.server)
	return a
}

func (a *Actions) Then() *Consequences {
	a.context.t.Helper()
	return &Consequences{a.context, a}
}

func (a *Actions) runCli(args ...string) {
	a.context.t.Helper()
	a.lastOutput, a.lastError = fixture.RunCli(args...)
}
