package helm

import (
	"fmt"
	"time"
)

type Entry struct {
	Version string
	Created time.Time
}

type Entries []Entry

func (es Entries) Tags() []string {
	tags := make([]string, len(es))
	for i, e := range es {
		tags[i] = e.Version
	}
	return tags
}

type Index struct {
	Entries map[string]Entries
}

func (i *Index) GetEntries(chart string) (Entries, error) {
	entries, ok := i.Entries[chart]
	if !ok {
		return nil, fmt.Errorf("chart '%s' not found in index", chart)
	}
	return entries, nil
}
