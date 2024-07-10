package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelmRepoAliasParsingWithSyntaxAt(t *testing.T) {
	repo := GetRepoNameFromAlias("@foo")
	assert.Equal(t, "foo", repo)
}

func TestHTestHelmRepoAliasParsingWithSyntaxAlias(t *testing.T) {
	repo := GetRepoNameFromAlias("alias:foo")
	assert.Equal(t, "foo", repo)
}

func TestHTestHelmRepoAliasParsingWithEmptyString(t *testing.T) {
	repo := GetRepoNameFromAlias("")
	assert.Equal(t, "", repo)
}

func TestHTestHelmRepoAliasParsingWithStandardUrl(t *testing.T) {
	repo := GetRepoNameFromAlias("https://example.com")
	assert.Equal(t, "", repo)
}
