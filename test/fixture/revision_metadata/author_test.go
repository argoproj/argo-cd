package revision_metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthor(t *testing.T) {
	t.Parallel()
	assert.Regexp(t, ".*<.*>", Author)
}
