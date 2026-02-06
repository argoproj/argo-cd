package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeLabels(t *testing.T) {
	inputLabels := []string{
		"already_normalized",
		"replace-dash",
		"replace$unallowed_char",
		"app.kubernetes.instance/test",
		"",
		"-starts-with_dash",
	}

	expectedNormalizedLabels := []string{
		"prefix_already_normalized",
		"prefix_replace_dash",
		"prefix_replace_unallowed_char",
		"prefix_app_kubernetes_instance_test",
		"prefix_",
		"prefix__starts_with_dash",
	}

	output := NormalizeLabels("prefix", inputLabels)
	assert.Equal(t, expectedNormalizedLabels, output)
}
