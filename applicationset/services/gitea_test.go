package services

import (
	"net/http"
	"testing"

	"code.gitea.io/sdk/gitea"
	"github.com/stretchr/testify/assert"
)

func TestGiteaAllCollected(t *testing.T) {
	t.Parallel()

	response := func(totalCount string) *gitea.Response {
		header := http.Header{}
		if totalCount != "" {
			header.Set("X-Total-Count", totalCount)
		}
		return &gitea.Response{Response: &http.Response{Header: header}}
	}

	tests := []struct {
		name      string
		resp      *gitea.Response
		collected int
		expected  bool
	}{
		{name: "no response", resp: nil, collected: 10, expected: false},
		{name: "header absent", resp: response(""), collected: 10, expected: false},
		{name: "header not a number", resp: response("many"), collected: 10, expected: false},
		{name: "fewer collected than total", resp: response("45"), collected: 40, expected: false},
		{name: "all collected", resp: response("45"), collected: 45, expected: true},
		{name: "more collected than total", resp: response("45"), collected: 50, expected: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.expected, GiteaAllCollected(test.resp, test.collected))
		})
	}
}
