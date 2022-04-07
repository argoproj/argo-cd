package pull_request

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGiteaList(t *testing.T) {
	host, err := NewGiteaService(context.Background(), "", "https://gitea.com", "test-argocd", "pr-test", false)
	assert.Nil(t, err)
	prs, err := host.List(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, len(prs), 1)
	assert.Equal(t, prs[0].Number, 1)
	assert.Equal(t, prs[0].Branch, "test")
	assert.Equal(t, prs[0].HeadSHA, "7bbaf62d92ddfafd9cc8b340c619abaec32bc09f")
}
