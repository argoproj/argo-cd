package tag

import (
	"encoding/hex"
	"sort"
	"sync"
	"time"

	"github.com/argoproj/argo-cd/v2/image-updater/log"

	"github.com/Masterminds/semver"
)

// ImageTag is a representation of an image tag with metadata
// Use NewImageTag to to initialize a new object.
type ImageTag struct {
	TagName   string
	TagDate   *time.Time
	TagDigest string
}

// ImageTagList is a collection of ImageTag objects.
// Use NewImageTagList to to initialize a new object.
type ImageTagList struct {
	items map[string]*ImageTag
	lock  *sync.RWMutex
}

// TagInfo contains information for a tag
type TagInfo struct {
	CreatedAt time.Time
	Digest    [32]byte
}

// SortableImageTagList is just that - a sortable list of ImageTag entries
type SortableImageTagList []*ImageTag

// Len returns the length of an SortableImageList
func (il SortableImageTagList) Len() int {
	return len(il)
}

// Swap swaps two entries in the SortableImageList
func (il SortableImageTagList) Swap(i, j int) {
	il[i], il[j] = il[j], il[i]
}

// NewImageTag initializes an ImageTag object and returns it
func NewImageTag(tagName string, tagDate time.Time, tagDigest string) *ImageTag {
	tag := &ImageTag{}
	tag.TagName = tagName
	tag.TagDate = &tagDate
	tag.TagDigest = tagDigest
	return tag
}

// NewImageTagList initializes an ImageTagList object and returns it
func NewImageTagList() *ImageTagList {
	itl := ImageTagList{}
	itl.items = make(map[string]*ImageTag)
	itl.lock = &sync.RWMutex{}
	return &itl
}

// Tags returns a list of verbatim tag names as string slice
func (il *ImageTagList) Tags() []string {
	il.lock.RLock()
	defer il.lock.RUnlock()
	tagList := []string{}
	for k := range il.items {
		tagList = append(tagList, k)
	}
	return tagList
}

// Tags returns a list of verbatim tag names as string slice
func (sil *SortableImageTagList) Tags() []string {
	tagList := []string{}
	for _, t := range *sil {
		tagList = append(tagList, t.TagName)
	}
	return tagList
}

// String returns the tag name of the ImageTag, possibly with a digest appended
// to its name.
func (tag *ImageTag) String() string {
	if tag.TagDigest != "" {
		return tag.TagDigest
	} else {
		return tag.TagName
	}
}

// IsDigest returns true if the tag has a digest
func (tag *ImageTag) IsDigest() bool {
	return tag.TagDigest != ""
}

// Equals checks whether two tags are equal. Will consider any digest set for
// the tag with precedence, otherwise uses a tag's name.
func (tag *ImageTag) Equals(aTag *ImageTag) bool {
	if tag.IsDigest() {
		return tag.TagDigest == aTag.TagDigest
	} else {
		return tag.TagName == aTag.TagName
	}
}

// Checks whether given tag is contained in tag list in O(n) time
func (il ImageTagList) Contains(tag *ImageTag) bool {
	il.lock.RLock()
	defer il.lock.RUnlock()
	return il.unlockedContains(tag)
}

// Add adds an ImageTag to an ImageTagList, ensuring this will not result in
// an double entry
func (il ImageTagList) Add(tag *ImageTag) {
	il.lock.Lock()
	defer il.lock.Unlock()
	il.items[tag.TagName] = tag
}

// SortByName returns an array of ImageTag objects, sorted by the tag's name
func (il ImageTagList) SortAlphabetically() SortableImageTagList {
	sil := SortableImageTagList{}
	for _, v := range il.items {
		sil = append(sil, v)
	}
	sort.Slice(sil, func(i, j int) bool {
		return sil[i].TagName < sil[j].TagName
	})
	return sil
}

// SortByDate returns a SortableImageTagList, sorted by the tag's date
func (il ImageTagList) SortByDate() SortableImageTagList {
	sil := SortableImageTagList{}
	for _, v := range il.items {
		sil = append(sil, v)
	}
	sort.Slice(sil, func(i, j int) bool {
		if sil[i].TagDate.Equal(*sil[j].TagDate) {
			// if an image has two tags, return the same consistently
			return sil[i].TagName < sil[j].TagName
		}
		return sil[i].TagDate.Before(*sil[j].TagDate)
	})
	return sil
}

func (il ImageTagList) SortBySemVer() SortableImageTagList {
	// We need a read lock, because we access the items hash after sorting
	il.lock.RLock()
	defer il.lock.RUnlock()

	sil := SortableImageTagList{}
	svl := make([]*semver.Version, 0)
	for _, v := range il.items {
		svi, err := semver.NewVersion(v.TagName)
		if err != nil {
			log.Debugf("could not parse input tag %s as semver: %v", v.TagName, err)
			continue
		}
		svl = append(svl, svi)
	}
	sort.Sort(semver.Collection(svl))
	for _, svi := range svl {
		sil = append(sil, NewImageTag(svi.Original(), *il.items[svi.Original()].TagDate, il.items[svi.Original()].TagDigest))
	}
	return sil
}

// Should only be used in a method that holds a lock on the ImageTagList
func (il ImageTagList) unlockedContains(tag *ImageTag) bool {
	if _, ok := il.items[tag.TagName]; ok {
		return true
	}
	return false
}

func (ti *TagInfo) EncodedDigest() string {
	return "sha256:" + hex.EncodeToString(ti.Digest[:])
}
