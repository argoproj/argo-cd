package annotation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	matchedAnnotations := []string{
		"argocd.argoproj.io/hook=PreSync",
	}

	parsedAnnotation := Parse(matchedAnnotations)
	t.Run("Matched Annotations", func(t *testing.T) {
		assert.Equal(t, "PreSync", parsedAnnotation["argocd.argoproj.io/hook"])
	})

	unMatchedAnnotation := []string{"argocd.argoproj.io/hook!=PreSync"}

	parsedAnnotation = Parse(unMatchedAnnotation)
	t.Run("Unmatched Annotations", func(t *testing.T) {
		assert.NotEqual(t, "PreSync", parsedAnnotation["argocd.argoproj.io/hook"])
	})

	var emptyAnnotations []string
	parsedAnnotation = Parse(emptyAnnotations)
	assert.Empty(t, parsedAnnotation)
}
