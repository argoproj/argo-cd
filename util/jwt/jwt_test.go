package jwt

import (
	"fmt"
	"testing"
	"time"

	jwt "github.com/dgrijalva/jwt-go/v4"
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

func TestIssuedAtTime_Int64(t *testing.T) {
	// Tuesday, 1 December 2020 14:00:00
	claims := jwt.MapClaims{"iat": int64(1606831200)}
	issuedAt, err := IssuedAtTime(claims)
	assert.Nil(t, err)
	str := fmt.Sprint(issuedAt.UTC().Format("Mon Jan _2 15:04:05 2006"))
	assert.Equal(t, "Tue Dec  1 14:00:00 2020", str)
}

func TestIssuedAtTime_Error_NoInt(t *testing.T) {
	claims := jwt.MapClaims{"iat": 1606831200}
	_, err := IssuedAtTime(claims)
	assert.NotNil(t, err)
}

func TestIssuedAtTime_Error_Missing(t *testing.T) {
	claims := jwt.MapClaims{}
	iat, err := IssuedAtTime(claims)
	assert.NotNil(t, err)
	assert.Equal(t, time.Unix(0, 0), iat)
}
