package helm

import (
	"fmt"
)

type Index struct {
	Tags map[string]TagsList
}

func (i *Index) GetTags(chart string) ([]string, error) {
	tags, ok := i.Tags[chart]
	if !ok {
		return nil, fmt.Errorf("chart '%s' not found in index", chart)
	}
	return tags.Tags, nil
}
