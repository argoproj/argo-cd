package jwt

import (
	"testing"
	"time"

	jwtgo "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSingleStringScope(t *testing.T) {
	claims := jwtgo.MapClaims{"groups": "my-org:my-team"}
	groups := GetScopeValues(claims, []string{"groups"})
	assert.Contains(t, groups, "my-org:my-team")
}

func TestGetMultipleListScopes(t *testing.T) {
	claims := jwtgo.MapClaims{"groups1": []string{"my-org:my-team1"}, "groups2": []string{"my-org:my-team2"}}
	groups := GetScopeValues(claims, []string{"groups1", "groups2"})
	assert.Contains(t, groups, "my-org:my-team1")
	assert.Contains(t, groups, "my-org:my-team2")
}

func TestClaims(t *testing.T) {
	assert.Nil(t, Claims(nil))
	assert.NotNil(t, Claims(jwtgo.MapClaims{}))
}

func TestIsMember(t *testing.T) {
	assert.False(t, IsMember(jwtgo.MapClaims{}, nil, []string{"groups"}))
	assert.False(t, IsMember(jwtgo.MapClaims{"groups": []string{""}}, []string{"my-group"}, []string{"groups"}))
	assert.False(t, IsMember(jwtgo.MapClaims{"groups": []string{"my-group"}}, []string{""}, []string{"groups"}))
	assert.True(t, IsMember(jwtgo.MapClaims{"groups": []string{"my-group"}}, []string{"my-group"}, []string{"groups"}))
}

func TestGetGroups(t *testing.T) {
	assert.Empty(t, GetGroups(jwtgo.MapClaims{}, []string{"groups"}))
	assert.Equal(t, []string{"foo"}, GetGroups(jwtgo.MapClaims{"groups": []string{"foo"}}, []string{"groups"}))
}

func TestIssuedAtTime_Int64(t *testing.T) {
	// Tuesday, 1 December 2020 14:00:00
	claims := jwtgo.MapClaims{"iat": int64(1606831200)}
	issuedAt, err := IssuedAtTime(claims)
	require.NoError(t, err)
	str := issuedAt.UTC().Format("Mon Jan _2 15:04:05 2006")
	assert.Equal(t, "Tue Dec  1 14:00:00 2020", str)
}

func TestIssuedAtTime_Error_NoInt(t *testing.T) {
	claims := jwtgo.MapClaims{"iat": 1606831200}
	_, err := IssuedAtTime(claims)
	assert.Error(t, err)
}

func TestIssuedAtTime_Error_Missing(t *testing.T) {
	claims := jwtgo.MapClaims{}
	iat, err := IssuedAtTime(claims)
	require.Error(t, err)
	assert.Equal(t, time.Unix(0, 0), iat)
}

func TestIsValid(t *testing.T) {
	assert.True(t, IsValid("foo.bar.foo"))
	assert.True(t, IsValid("foo.bar.foo.bar"))
	assert.False(t, IsValid("foo.bar"))
	assert.False(t, IsValid("foo"))
	assert.False(t, IsValid(""))
}
