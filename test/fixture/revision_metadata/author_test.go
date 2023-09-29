package revision_metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthor(t *testing.T) {
	assert.Regexp(t, ".*<.*>", Author)
}
