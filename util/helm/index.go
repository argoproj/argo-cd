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

// GetChartURL returns the first URL for the given chart and version (e.g. for provenance .prov fetch).
func (i *Index) GetChartURL(chart string, version string) (string, error) {
	entries, err := i.GetEntries(chart)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.Version == version && len(e.Urls) > 0 {
			return e.Urls[0], nil
		}
	}
	return "", fmt.Errorf("chart '%s' version '%s' not found in index", chart, version)
}
