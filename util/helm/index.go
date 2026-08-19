package helm

import (
	"fmt"
	"time"
)

type Entry struct {
	Version string
	Created time.Time
	// Urls are the chart package URLs from the index (e.g. for provenance .prov fetch).
	Urls []string
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

// GetChartURLs returns all chart package URLs for the given chart and version (mirrors per Helm index spec).
func (i *Index) GetChartURLs(chart string, version string) ([]string, error) {
	entries, err := i.GetEntries(chart)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.Version == version && len(e.Urls) > 0 {
			return append([]string(nil), e.Urls...), nil
		}
	}
	return nil, fmt.Errorf("chart '%s' version '%s' not found in index", chart, version)
}
