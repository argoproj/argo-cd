package repos

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelmClient_Checkout(t *testing.T) {
	tmp, err := ioutil.TempDir("", "helm")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmp) }()

	client, err := factory{}.newHelmClient("https://kubernetes-charts.storage.googleapis.com", tmp, "", "", nil, nil, nil)
	assert.NoError(t, err)
	assert.NoError(t, client.Test())

	resolvedRevision, err := client.ResolveRevision("5.4.0")
	assert.NoError(t, err)
	assert.Equal(t, "5.4.0", resolvedRevision)

	_, err = client.Checkout("wordpress", resolvedRevision)
	assert.NoError(t, err)

	checkedOutRevision, err := client.Checkout("wordpress", resolvedRevision)
	assert.NoError(t, err)
	assert.Equal(t, resolvedRevision, checkedOutRevision)
}
