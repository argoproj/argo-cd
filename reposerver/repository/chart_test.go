package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_getChartDetailsNotSet(t *testing.T) {
	chart1 := `apiVersion: v3
name: mychart
version: 0.0.0`

	cd, err := getChartDetails(chart1)
	require.NoError(t, err)
	assert.Equal(t, "", cd.Description)
	assert.Equal(t, cd.Maintainers, []string(nil))
	assert.Equal(t, "", cd.Home)
}

func Test_getChartDetailsSet(t *testing.T) {
	chart1 := `apiVersion: v3
name: mychart
version: 0.0.0
description: a good chart
home: https://example.com
maintainers:
- name: alex
  email: example@example.com
`

	cd, err := getChartDetails(chart1)
	require.NoError(t, err)
	assert.Equal(t, "a good chart", cd.Description)
	assert.Equal(t, []string{"alex <example@example.com>"}, cd.Maintainers)
	assert.Equal(t, "https://example.com", cd.Home)

	chart1 = `apiVersion: v3
name: mychart
version: 0.0.0
description: a good chart
home: https://example.com
maintainers:
- name: alex
`
	cd, err = getChartDetails(chart1)
	require.NoError(t, err)
	assert.Equal(t, []string{"alex"}, cd.Maintainers)
}

func Test_getChartDetailsBad(t *testing.T) {
	chart1 := `apiVersion: v3
name: mychart
version: 0.0.0
description: a good chart
home: https://example.com
maintainers: alex
`

	cd, err := getChartDetails(chart1)
	require.Error(t, err)
	assert.Nil(t, cd)
}
