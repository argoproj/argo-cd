package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getChartDetailsNotSet(t *testing.T) {
	chart1 := `apiVersion: v3
name: mychart
version: 0.0.0`

	cd, err := getChartDetails(chart1)
	assert.NoError(t, err)
	assert.Equal(t, cd.Description, "")
	assert.Equal(t, cd.Maintainers, []string(nil))
	assert.Equal(t, cd.Home, "")
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
	assert.NoError(t, err)
	assert.Equal(t, cd.Description, "a good chart")
	assert.Equal(t, cd.Maintainers, []string{"alex <example@example.com>"})
	assert.Equal(t, cd.Home, "https://example.com")

	chart1 = `apiVersion: v3
name: mychart
version: 0.0.0
description: a good chart
home: https://example.com
maintainers:
- name: alex
`
	cd, err = getChartDetails(chart1)
	assert.NoError(t, err)
	assert.Equal(t, cd.Maintainers, []string{"alex"})
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
	assert.Error(t, err)
	assert.Nil(t, cd)
}
