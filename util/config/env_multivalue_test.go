package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func set(t *testing.T, val string) {
	t.Helper()
	t.Setenv("ARGOCD_OPTS", val)
	assert.NoError(t, LoadFlags())
}

func TestMultiHeaderAccumulation(t *testing.T) {
	set(t, `--header 'CF-Access-Client-Id: foo' --header 'CF-Access-Client-Secret: bar'`)
	got := GetStringSliceFlag("header", []string{})
	assert.ElementsMatch(t, []string{
		"CF-Access-Client-Id: foo",
		"CF-Access-Client-Secret: bar",
	}, got)
}

func TestHeaderEqualsForm(t *testing.T) {
	set(t, `--header=CF-Access-Client-Id:foo --header=CF-Access-Client-Secret:bar`)
	got := GetStringSliceFlag("header", []string{})
	assert.ElementsMatch(t, []string{
		"CF-Access-Client-Id:foo",
		"CF-Access-Client-Secret:bar",
	}, got)
}

func TestCsvSingleOccurrenceStillParses(t *testing.T) {
	set(t, `--header 'Content-Type: application/json; charset=utf-8,Strict-Transport-Security: max-age=31536000'`)
	got := GetStringSliceFlag("header", []string{})
	assert.Equal(t, []string{
		"Content-Type: application/json; charset=utf-8",
		"Strict-Transport-Security: max-age=31536000",
	}, got)
}

func TestOtherFlagsUnchanged(t *testing.T) {
	set(t, `--loglevel debug --loglevel info --retries 3`)
	assert.Equal(t, "info", GetFlag("loglevel", ""))
	assert.Equal(t, 3, GetIntFlag("retries", 0))
	assert.False(t, GetBoolFlag("dryrun"))
}
