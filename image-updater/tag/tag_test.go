package tag

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewImageTag(t *testing.T) {
	t.Run("New image tag from valid Time type", func(t *testing.T) {
		tagDate := time.Now()
		tag := NewImageTag("v1.0.0", tagDate, "")
		require.NotNil(t, tag)
		assert.Equal(t, "v1.0.0", tag.TagName)
		assert.Equal(t, &tagDate, tag.TagDate)
	})
}

func Test_ImageTagEqual(t *testing.T) {
	t.Run("Versions are similar", func(t *testing.T) {
		tag1 := NewImageTag("v1.0.0", time.Now(), "")
		tag2 := NewImageTag("v1.0.0", time.Now(), "")
		assert.True(t, tag1.Equals(tag2))
	})

	t.Run("Digests are similar but version is not", func(t *testing.T) {
		tag1 := NewImageTag("v1.0.0", time.Now(), "abcdef")
		tag2 := NewImageTag("v1.0.1", time.Now(), "abcdef")
		assert.True(t, tag1.Equals(tag2))
	})

	t.Run("Digests and versions are similar", func(t *testing.T) {
		tag1 := NewImageTag("v1.0.0", time.Now(), "abcdef")
		tag2 := NewImageTag("v1.0.0", time.Now(), "abcdef")
		assert.True(t, tag1.Equals(tag2))
	})

	t.Run("Versions are not similar", func(t *testing.T) {
		tag1 := NewImageTag("v1.0.0", time.Now(), "")
		tag2 := NewImageTag("v1.0.1", time.Now(), "")
		assert.False(t, tag1.Equals(tag2))
	})

	t.Run("Versions are not similar because digest is different", func(t *testing.T) {
		tag1 := NewImageTag("v1.0.0", time.Now(), "abc")
		tag2 := NewImageTag("v1.0.0", time.Now(), "def")
		assert.False(t, tag1.Equals(tag2))
	})

	t.Run("Versions are not similar because digest is missing", func(t *testing.T) {
		tag1 := NewImageTag("v1.0.0", time.Now(), "abc")
		tag2 := NewImageTag("v1.0.0", time.Now(), "")
		assert.False(t, tag1.Equals(tag2))
	})

}

func Test_AppendToImageTagList(t *testing.T) {
	t.Run("Append single entry to ImageTagList", func(t *testing.T) {
		il := NewImageTagList()
		tag := NewImageTag("v1.0.0", time.Now(), "")
		il.Add(tag)
		assert.Len(t, il.items, 1)
		assert.True(t, il.Contains(tag))
	})

	t.Run("Append two same entries to ImageTagList", func(t *testing.T) {
		il := NewImageTagList()
		tag := NewImageTag("v1.0.0", time.Now(), "")
		il.Add(tag)
		tag = NewImageTag("v1.0.0", time.Now(), "")
		il.Add(tag)
		assert.True(t, il.Contains(tag))
		assert.Len(t, il.items, 1)
	})

	t.Run("Append two distinct entries to ImageTagList", func(t *testing.T) {
		il := NewImageTagList()
		tag1 := NewImageTag("v1.0.0", time.Now(), "")
		il.Add(tag1)
		tag2 := NewImageTag("v1.0.1", time.Now(), "")
		il.Add(tag2)
		assert.True(t, il.Contains(tag1))
		assert.True(t, il.Contains(tag2))
		assert.Len(t, il.items, 2)
	})
}

func Test_SortableImageTagList(t *testing.T) {
	t.Run("Sort by name", func(t *testing.T) {
		names := []string{"wohoo", "bazar", "alpha", "jesus", "zebra"}
		il := NewImageTagList()
		for _, name := range names {
			tag := NewImageTag(name, time.Now(), "")
			il.Add(tag)
		}
		sil := il.SortAlphabetically()
		require.Len(t, sil, len(names))
		assert.Equal(t, "alpha", sil[0].TagName)
		assert.Equal(t, "bazar", sil[1].TagName)
		assert.Equal(t, "jesus", sil[2].TagName)
		assert.Equal(t, "wohoo", sil[3].TagName)
		assert.Equal(t, "zebra", sil[4].TagName)
	})

	t.Run("Sort by semver", func(t *testing.T) {
		names := []string{"v2.0.2", "v1.0", "v1.0.1", "v2.0.3", "v2.0"}
		il := NewImageTagList()
		for _, name := range names {
			tag := NewImageTag(name, time.Now(), "")
			il.Add(tag)
		}
		sil := il.SortBySemVer()
		require.Len(t, sil, len(names))
		assert.Equal(t, "v1.0", sil[0].TagName)
		assert.Equal(t, "v1.0.1", sil[1].TagName)
		assert.Equal(t, "v2.0", sil[2].TagName)
		assert.Equal(t, "v2.0.2", sil[3].TagName)
		assert.Equal(t, "v2.0.3", sil[4].TagName)
	})

	t.Run("Sort by date", func(t *testing.T) {
		names := []string{"v2.0.2", "v1.0", "v1.0.1", "v2.0.3", "v2.0"}
		dates := []int64{4, 1, 0, 3, 2}
		il := NewImageTagList()
		for i, name := range names {
			tag := NewImageTag(name, time.Unix(dates[i], 0), "")
			il.Add(tag)
		}
		sil := il.SortByDate()
		require.Len(t, sil, len(names))
		assert.Equal(t, "v1.0.1", sil[0].TagName)
		assert.Equal(t, "v1.0", sil[1].TagName)
		assert.Equal(t, "v2.0", sil[2].TagName)
		assert.Equal(t, "v2.0.3", sil[3].TagName)
		assert.Equal(t, "v2.0.2", sil[4].TagName)
	})

	t.Run("Sort by date with same dates", func(t *testing.T) {
		names := []string{"v2.0.2", "v1.0", "v1.0.1", "v2.0.3", "v2.0"}
		date := time.Unix(0, 0)
		il := NewImageTagList()
		for _, name := range names {
			tag := NewImageTag(name, date, "")
			il.Add(tag)
		}
		sil := il.SortByDate()
		require.Len(t, sil, len(names))
		assert.Equal(t, "v1.0", sil[0].TagName)
		assert.Equal(t, "v1.0.1", sil[1].TagName)
		assert.Equal(t, "v2.0", sil[2].TagName)
		assert.Equal(t, "v2.0.2", sil[3].TagName)
		assert.Equal(t, "v2.0.3", sil[4].TagName)
	})
}

func Test_TagsFromTagList(t *testing.T) {
	t.Run("Get list of tags from ImageTagList", func(t *testing.T) {
		names := []string{"wohoo", "bazar", "alpha", "jesus", "zebra"}
		il := NewImageTagList()
		for _, name := range names {
			tag := NewImageTag(name, time.Now(), "")
			il.Add(tag)
		}
		tl := il.Tags()
		assert.NotEmpty(t, tl)
		assert.Len(t, tl, len(names))
	})

	t.Run("Get list of tags from SortableImageTagList", func(t *testing.T) {
		names := []string{"wohoo", "bazar", "alpha", "jesus", "zebra"}
		sil := SortableImageTagList{}
		for _, name := range names {
			tag := NewImageTag(name, time.Now(), "")
			sil = append(sil, tag)
		}
		tl := sil.Tags()
		assert.NotEmpty(t, tl)
		assert.Len(t, tl, len(names))
	})
}
