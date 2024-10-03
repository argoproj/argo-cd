package helm

import (
	"fmt"
)

type Index struct {
	Entries map[string]Entries
}

func (i *Index) GetTags(chart string) ([]string, error) {
	tags, ok := i.Entries[chart]
	if !ok {
		return nil, fmt.Errorf("chart '%s' not found in index", chart)
	}
	return tags.Tags, nil
}
