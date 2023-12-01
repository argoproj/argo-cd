package featureflag_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/argoproj/argo-cd/v2/util/featureflag"
	"github.com/stretchr/testify/assert"
)

type FeatureFlager interface {
	IsEnabled() bool
	Enable()
	Disable()
	Description() string
}

func TestFeatureFlags(t *testing.T) {

	t.Run("will validate a simple usage", func(t *testing.T) {
		// Given
		ff := featureflag.New()

		// Then
		assert.False(t, ff.ExampleFeature.IsEnabled(), "ExampleFeature flags should always be disabled")
		assert.NotEmpty(t, ff.ExampleFeature.Description(), "All feature flags must provide a description")
		ff.ExampleFeature.Enable()
		assert.True(t, ff.ExampleFeature.IsEnabled(), "ExampleFeature should be enabled")
	})
	t.Run("will validate that all feature flags are properly initialized", func(t *testing.T) {
		// Given
		ff := featureflag.New()
		value := reflect.ValueOf(ff)
		types := value.Type()
		fields := make(map[string]interface{}, value.NumField())

		// When
		for i := 0; i < value.NumField(); i++ {
			if value.Field(i).CanInterface() {
				fields[types.Field(i).Name] = value.Field(i).Interface()
			}
		}

		// Then
		for k, v := range fields {
			assert.NotNil(t, v, fmt.Sprintf("%s feature flag needs to be initialized in the New function", k))
			if ff, ok := v.(FeatureFlager); ok {
				assert.NotEmpty(t, ff.Description(), fmt.Sprintf("%s feature flag needs to define its description", k))
			}
		}
	})
}
