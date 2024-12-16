//go:build !race
// +build !race

package application

import (
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/ptr"
	"testing"
)

func TestMaxPodLogsRender(t *testing.T) {
	// !race:
	// Intermittent failure when running TestMaxPodLogsRender with -race, likely due to race condition
	// https://github.com/argoproj/argo-cd/issues/4755
	defaultMaxPodLogsToRender, _ := newTestAppServer(t).settingsMgr.GetMaxPodLogsToRender()

	// Case: number of pods to view logs is less than defaultMaxPodLogsToRender
	podNumber := int(defaultMaxPodLogsToRender - 1)
	appServer, adminCtx := createAppServerWithMaxLodLogs(t, podNumber)

	t.Run("PodLogs", func(t *testing.T) {
		err := appServer.PodLogs(&application.ApplicationPodLogsQuery{Name: ptr.To("test")}, &TestPodLogsServer{ctx: adminCtx})
		statusCode, _ := status.FromError(err)
		assert.Equal(t, codes.OK, statusCode.Code())
	})

	// Case: number of pods higher than defaultMaxPodLogsToRender
	podNumber = int(defaultMaxPodLogsToRender + 1)
	appServer, adminCtx = createAppServerWithMaxLodLogs(t, podNumber)

	t.Run("PodLogs", func(t *testing.T) {
		err := appServer.PodLogs(&application.ApplicationPodLogsQuery{Name: ptr.To("test")}, &TestPodLogsServer{ctx: adminCtx})
		require.Error(t, err)
		statusCode, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, statusCode.Code())
		assert.EqualError(t, err, "rpc error: code = InvalidArgument desc = max pods to view logs are reached. Please provide more granular query")
	})

	// Case: number of pods to view logs is less than customMaxPodLogsToRender
	customMaxPodLogsToRender := int64(15)
	podNumber = int(customMaxPodLogsToRender - 1)
	appServer, adminCtx = createAppServerWithMaxLodLogs(t, podNumber, customMaxPodLogsToRender)

	t.Run("PodLogs", func(t *testing.T) {
		err := appServer.PodLogs(&application.ApplicationPodLogsQuery{Name: ptr.To("test")}, &TestPodLogsServer{ctx: adminCtx})
		statusCode, _ := status.FromError(err)
		assert.Equal(t, codes.OK, statusCode.Code())
	})

	// Case: number of pods higher than customMaxPodLogsToRender
	customMaxPodLogsToRender = int64(15)
	podNumber = int(customMaxPodLogsToRender + 1)
	appServer, adminCtx = createAppServerWithMaxLodLogs(t, podNumber, customMaxPodLogsToRender)

	t.Run("PodLogs", func(t *testing.T) {
		err := appServer.PodLogs(&application.ApplicationPodLogsQuery{Name: ptr.To("test")}, &TestPodLogsServer{ctx: adminCtx})
		require.Error(t, err)
		statusCode, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, statusCode.Code())
		assert.EqualError(t, err, "rpc error: code = InvalidArgument desc = max pods to view logs are reached. Please provide more granular query")
	})
}
