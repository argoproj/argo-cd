package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func Test_generateRule(t *testing.T) {
	t.Run("GenerateLabelRule", func(t *testing.T) {
		rule, err := generateRule([]string{"test in thisLabel"})
		assert.Nil(t, err)
		assert.Equal(t, rule.Conditions[0].Kind, argoappv1.ConditionKindLabel)
		assert.Equal(t, rule.Conditions[0].Operator, argoappv1.ConditionOperatorIn)
		assert.Equal(t, rule.Conditions[0].Key, "test")
		assert.Equal(t, rule.Conditions[0].Values[0], "thisLabel")
	})
	t.Run("GenerateLabelExistsRule", func(t *testing.T) {
		rule, err := generateRule([]string{"test exists"})
		assert.Nil(t, err)
		assert.Equal(t, rule.Conditions[0].Kind, argoappv1.ConditionKindLabel)
		assert.Equal(t, rule.Conditions[0].Operator, argoappv1.ConditionOperatorExists)
		assert.Equal(t, rule.Conditions[0].Key, "test")
		assert.Equal(t, rule.Conditions[0].Values[0], "")
	})
	t.Run("GenerateApplicationRule", func(t *testing.T) {
		rule, err  := generateRule([]string{"application in thisApp"})
		assert.Nil(t, err)
		assert.Equal(t, rule.Conditions[0].Kind, argoappv1.ConditionKindApplication)
		assert.Equal(t, rule.Conditions[0].Operator, argoappv1.ConditionOperatorIn)
		assert.Equal(t, rule.Conditions[0].Key, "")
		assert.Equal(t, rule.Conditions[0].Values[0], "thisApp")
	})
	t.Run("GenerateNamespaceRule", func(t *testing.T) {
		rule, err  := generateRule([]string{"namespace notIn thisNamespace"})
		assert.Nil(t, err)
		assert.Equal(t, rule.Conditions[0].Kind, argoappv1.ConditionKindNamespace)
		assert.Equal(t, rule.Conditions[0].Operator, argoappv1.ConditionOperatorNotIn)
		assert.Equal(t, rule.Conditions[0].Key, "")
		assert.Equal(t, rule.Conditions[0].Values[0], "thisNamespace")
	})
	t.Run("GenerateTooManyFieldsIn", func(t *testing.T) {
		_, err := generateRule([]string{"cluster is in thisCluster"})
		assert.Contains(t, err.Error(), "field mismatch expected 3 got 4")
	})
	t.Run("GenerateTooManyFieldsExists", func(t *testing.T) {
		_, err := generateRule([]string{"cluster 1 exists"})
		assert.Contains(t, err.Error(), "field mismatch expected 2 got 3")
	})
	t.Run("GenerateIncorrectOperator", func(t *testing.T) {
		_, err := generateRule([]string{"cluster shouldBe cluster1"})
		assert.Contains(t, err.Error(), "operator 'shouldBe' not supported")
	})
}

