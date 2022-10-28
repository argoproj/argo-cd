package pull_request

import (
	"testing"

	"github.com/google/go-github/v35/github"
)

func toPtr(s string) *string {
	return &s
}

func TestContainLabels(t *testing.T) {
	cases := []struct {
		Name       string
		Labels     []string
		PullLabels []*github.Label
		Expect     bool
	}{
		{
			Name:   "Match labels",
			Labels: []string{"label1", "label2"},
			PullLabels: []*github.Label{
				&github.Label{Name: toPtr("label1")},
				&github.Label{Name: toPtr("label2")},
				&github.Label{Name: toPtr("label3")},
			},
			Expect: true,
		},
		{
			Name:   "Not match labels",
			Labels: []string{"label1", "label4"},
			PullLabels: []*github.Label{
				&github.Label{Name: toPtr("label1")},
				&github.Label{Name: toPtr("label2")},
				&github.Label{Name: toPtr("label3")},
			},
			Expect: false,
		},
		{
			Name:   "No specify",
			Labels: []string{},
			PullLabels: []*github.Label{
				&github.Label{Name: toPtr("label1")},
				&github.Label{Name: toPtr("label2")},
				&github.Label{Name: toPtr("label3")},
			},
			Expect: true,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if got := containLabels(c.Labels, c.PullLabels); got != c.Expect {
				t.Errorf("expect: %v, got: %v", c.Expect, got)
			}
		})
	}
}
