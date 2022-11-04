package image

import (
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/image-updater/tag"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newImageTagList(tagNames []string) *tag.ImageTagList {
	tagList := tag.NewImageTagList()
	for _, tagName := range tagNames {
		tagList.Add(tag.NewImageTag(tagName, time.Unix(0, 0), ""))
	}
	return tagList
}

func newImageTagListWithDate(tagNames []string) *tag.ImageTagList {
	tagList := tag.NewImageTagList()
	for i, t := range tagNames {
		tagList.Add(tag.NewImageTag(t, time.Unix(int64(i*5), 0), ""))
	}
	return tagList
}

func Test_LatestVersion(t *testing.T) {
	t.Run("Find the latest version without any constraint", func(t *testing.T) {
		tagList := newImageTagList([]string{"0.1", "0.5.1", "0.9", "1.0", "1.0.1", "1.1.2", "2.0.3"})
		img := NewFromIdentifier("jannfis/test:1.0")
		vc := VersionConstraint{}
		newTag, err := img.GetNewestVersionFromTags(&vc, tagList)
		require.NoError(t, err)
		require.NotNil(t, newTag)
		assert.Equal(t, "2.0.3", newTag.TagName)
	})

	t.Run("Find the latest version with a semver constraint on major", func(t *testing.T) {
		tagList := newImageTagList([]string{"0.1", "0.5.1", "0.9", "1.0", "1.0.1", "1.1.2", "2.0.3"})
		img := NewFromIdentifier("jannfis/test:1.0")
		vc := VersionConstraint{Constraint: "^1.0"}
		newTag, err := img.GetNewestVersionFromTags(&vc, tagList)
		require.NoError(t, err)
		require.NotNil(t, newTag)
		assert.Equal(t, "1.1.2", newTag.TagName)
	})

	t.Run("Find the latest version with a semver constraint on patch", func(t *testing.T) {
		tagList := newImageTagList([]string{"0.1", "0.5.1", "0.9", "1.0", "1.0.1", "1.1.2", "2.0.3"})
		img := NewFromIdentifier("jannfis/test:1.0")
		vc := VersionConstraint{Constraint: "~1.0"}
		newTag, err := img.GetNewestVersionFromTags(&vc, tagList)
		require.NoError(t, err)
		require.NotNil(t, newTag)
		assert.Equal(t, "1.0.1", newTag.TagName)
	})

	t.Run("Find the latest version with a semver constraint that has no match", func(t *testing.T) {
		tagList := newImageTagList([]string{"0.1", "0.5.1", "0.9", "2.0.3"})
		img := NewFromIdentifier("jannfis/test:1.0")
		vc := VersionConstraint{Constraint: "~1.0"}
		newTag, err := img.GetNewestVersionFromTags(&vc, tagList)
		require.NoError(t, err)
		require.Nil(t, newTag)
	})

	t.Run("Find the latest version with a semver constraint that is invalid", func(t *testing.T) {
		tagList := newImageTagList([]string{"0.1", "0.5.1", "0.9", "2.0.3"})
		img := NewFromIdentifier("jannfis/test:1.0")
		vc := VersionConstraint{Constraint: "latest"}
		newTag, err := img.GetNewestVersionFromTags(&vc, tagList)
		assert.Error(t, err)
		assert.Nil(t, newTag)
	})

	t.Run("Find the latest version with no tags", func(t *testing.T) {
		tagList := newImageTagList([]string{})
		img := NewFromIdentifier("jannfis/test:1.0")
		vc := VersionConstraint{Constraint: "~1.0"}
		newTag, err := img.GetNewestVersionFromTags(&vc, tagList)
		require.NoError(t, err)
		require.NotNil(t, newTag)
		assert.Equal(t, "1.0", newTag.TagName)
	})

	t.Run("Find the latest version using latest sortmode", func(t *testing.T) {
		tagList := newImageTagListWithDate([]string{"zz", "bb", "yy", "cc", "yy", "aa", "ll"})
		img := NewFromIdentifier("jannfis/test:bb")
		vc := VersionConstraint{Strategy: StrategyNewestBuild}
		newTag, err := img.GetNewestVersionFromTags(&vc, tagList)
		require.NoError(t, err)
		require.NotNil(t, newTag)
		assert.Equal(t, "ll", newTag.TagName)
	})

	t.Run("Find the latest version using latest sortmode, invalid tags", func(t *testing.T) {
		tagList := newImageTagListWithDate([]string{"zz", "bb", "yy", "cc", "yy", "aa", "ll"})
		img := NewFromIdentifier("jannfis/test:bb")
		vc := VersionConstraint{Strategy: StrategySemVer}
		newTag, err := img.GetNewestVersionFromTags(&vc, tagList)
		require.NoError(t, err)
		require.NotNil(t, newTag)
		assert.Equal(t, "bb", newTag.TagName)
	})

}
