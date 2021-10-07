package argo

import (
	"context"
	"fmt"

	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/db"
)

func GetCluster(db db.ArgoDB, ctx context.Context, server string) (*argoappv1.Cluster, error) {
	cluster, err := db.GetCluster(ctx, server)
	if cluster != nil {
		return cluster, nil
	}
	clusterList, err := db.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range clusterList.Items {
		if c.Name == server {
			return &c, nil
		}
	}
	return nil, fmt.Errorf("cluster not found")

}
