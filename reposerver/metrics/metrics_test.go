package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	factorymocks "github.com/argoproj/argo-cd/util/repo/factory/mocks"
	repomocks "github.com/argoproj/argo-cd/util/repo/mocks"
)

func TestNewMetricsServer(t *testing.T) {
	factory := &factorymocks.Factory{}
	repo := &repomocks.Repo{}
	factory.On("NewRepo", mock.Anything, mock.Anything).Return(repo, nil)
	server := NewMetricsServer(factory)
	_, err := server.NewRepo(nil, nil)
	assert.NoError(t, err)
	server.Event("foo", "GitRequestTypeFetch")
	counter, err := server.gitRequestCounter.GetMetricWithLabelValues("foo", "fetch")
	assert.NoError(t, err)
	assert.NotNil(t, counter)
	server.Event("bar", "GitRequestTypeLsRemote")
	counter, err = server.gitRequestCounter.GetMetricWithLabelValues("foo", "ls-remote")
	assert.NoError(t, err)
	assert.NotNil(t, counter)
}
