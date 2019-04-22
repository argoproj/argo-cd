package repository

import (
	"reflect"
	"testing"

	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/rbac"
	"golang.org/x/net/context"
)

func TestServer_getConnectionState(t *testing.T) {
	type fields struct {
		db            db.ArgoDB
		repoClientset reposerver.Clientset
		enf           *rbac.Enforcer
		cache         *cache.Cache
	}
	type args struct {
		ctx context.Context
		url string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   appsv1.ConnectionState
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				db:            tt.fields.db,
				repoClientset: tt.fields.repoClientset,
				enf:           tt.fields.enf,
				cache:         tt.fields.cache,
			}
			if got := s.getConnectionState(tt.args.ctx, tt.args.url); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Server.getConnectionState() = %v, want %v", got, tt.want)
			}
		})
	}
}
