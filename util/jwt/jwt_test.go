package jwt

import (
	"testing"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"
)

func TestGetSingleStringScope(t *testing.T) {
	claims := jwt.MapClaims{"groups": "my-org:my-team"}
	groups := GetScopeValues(claims, []string{"groups"})
	assert.Contains(t, groups, "my-org:my-team")
}

func TestGetMultipleListScopes(t *testing.T) {
	claims := jwt.MapClaims{"groups1": []string{"my-org:my-team1"}, "groups2": []string{"my-org:my-team2"}}
	groups := GetScopeValues(claims, []string{"groups1", "groups2"})
	assert.Contains(t, groups, "my-org:my-team1")
	assert.Contains(t, groups, "my-org:my-team2")
}

func TestClaims(t *testing.T) {
	assert.Nil(t, Claims(nil))
	assert.NotNil(t, Claims(jwt.MapClaims{}))
}

func TestIsMember(t *testing.T) {
	assert.False(t, IsMember(jwt.MapClaims{}, nil, []string{"groups"}))
	assert.False(t, IsMember(jwt.MapClaims{"groups": []string{""}}, []string{"my-group"}, []string{"groups"}))
	assert.False(t, IsMember(jwt.MapClaims{"groups": []string{"my-group"}}, []string{""}, []string{"groups"}))
	assert.True(t, IsMember(jwt.MapClaims{"groups": []string{"my-group"}}, []string{"my-group"}, []string{"groups"}))
}

func TestGetGroups(t *testing.T) {
	assert.Empty(t, GetGroups(jwt.MapClaims{}, []string{"groups"}))
	assert.Equal(t, []string{"foo"}, GetGroups(jwt.MapClaims{"groups": []string{"foo"}}, []string{"groups"}))
}
