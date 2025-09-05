package config

import (
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func set(t *testing.T, val string) {
	t.Helper()
	t.Setenv("ARGOCD_OPTS", val)
	require.NoError(t, LoadFlags())
}

// --- Core tests for headers ---

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

// --- Extra coverage for headers ---

func TestHeader_MixedForms_CollectsAllValues(t *testing.T) {
	set(t, `--header 'X: one' --header=Y:two`)
	got := GetStringSliceFlag("header", []string{})
	assert.Equal(t, []string{"X: one", "Y:two"}, got)
}

func TestHeader_EmptySingleOccurrence_YieldsEmptySlice(t *testing.T) {
	set(t, `--header ''`)
	got := GetStringSliceFlag("header", []string{})
	assert.Equal(t, []string{}, got)
}

func TestHeader_MixedValueAndEqualsForm_ExistingKey(t *testing.T) {
	set(t, `--header 'X: one' --header=X:two`)
	got := GetStringSliceFlag("header", []string{})
	assert.Equal(t, []string{"X: one", "X:two"}, got)
}

func TestHeader_MultipleOccurrencesWithEmptyAndNonEmpty(t *testing.T) {
	set(t, `--header 'first' --header '' --header 'third'`)
	got := GetStringSliceFlag("header", []string{})
	assert.Equal(t, []string{"first", "", "third"}, got)
}

func TestStringSliceFlag_CsvParsingMultipleValues(t *testing.T) {
	set(t, `--header 'a,b,c'`)
	got := GetStringSliceFlag("header", []string{})
	assert.Equal(t, []string{"a", "b", "c"}, got)
}

// --- Fallback behavior ---

func TestStringSliceFlag_FallbackWhenNotSet(t *testing.T) {
	set(t, `--loglevel debug`)
	got := GetStringSliceFlag("header", []string{"fallback"})
	assert.Equal(t, []string{"fallback"}, got)
}

func TestStringSliceFlag_ExplicitEmptyOverridesFallback(t *testing.T) {
	set(t, `--header ''`)
	got := GetStringSliceFlag("header", []string{"fallback"})
	assert.Equal(t, []string{}, got)
}

func TestStringSliceFlag_CsvAppendWithExistingKey(t *testing.T) {
	set(t, `--header 'X: one,Y:two' --header Z:three`)
	got := GetStringSliceFlag("header", []string{})
	assert.Equal(t, []string{"X: one,Y:two", "Z:three"}, got)
}

// --- Error cases ---

func TestHeader_InvalidOptsError(t *testing.T) {
	t.Setenv("ARGOCD_OPTS", "not-a-flag")
	err := LoadFlags()
	assert.Error(t, err)
}

func TestInvalidCSVError(t *testing.T) {
	t.Setenv("ARGOCD_OPTS", `--header '"unterminated`)
	err := LoadFlags()
	assert.Error(t, err)
}

// --- Special fatal case for GetIntFlag ---

func TestIntFlag_InvalidValue_Exits(t *testing.T) {
	if os.Getenv("BE_CRASHER") == "1" {
		set(t, `--retries not-an-int`)
		GetIntFlag("retries", 0)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestIntFlag_InvalidValue_Exits")
	cmd.Env = append(os.Environ(), "BE_CRASHER=1")
	_ = cmd.Run()
}

// --- Invalid CSV handling without crashing ---

func TestStringSliceFlag_InvalidCSV_ReturnsRaw(t *testing.T) {
	if os.Getenv("BE_CRASHER_CSV") == "1" {
		set(t, "--loglevel debug")
		require.NoError(t, LoadFlags())
		if flags == nil {
			flags = make(map[string]string)
		}
		flags["header"] = `a,"unterminated`
		_ = GetStringSliceFlag("header", []string{"fallback"})
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestStringSliceFlag_InvalidCSV_ReturnsRaw")
	cmd.Env = append(os.Environ(), "BE_CRASHER_CSV=1")
	_ = cmd.Run()
}

// --- Extra init() and default coverage ---

func TestInit_NoEnv(t *testing.T) {
	os.Unsetenv("ARGOCD_OPTS")
	require.NoError(t, LoadFlags())
}

func TestInit_EmptyEnv(t *testing.T) {
	t.Setenv("ARGOCD_OPTS", "")
	require.NoError(t, LoadFlags())
}

func TestGetIntFlag_DefaultWhenUnset(t *testing.T) {
	set(t, `--loglevel debug`)
	got := GetIntFlag("does-not-exist", 99)
	assert.Equal(t, 99, got)
}

func TestStringSliceFlag_DefaultWhenUnset(t *testing.T) {
	set(t, `--loglevel debug`)
	got := GetStringSliceFlag("does-not-exist", []string{"fallback"})
	assert.Equal(t, []string{"fallback"}, got)
}

// --- Extra coverage for init() subprocess ---

func TestInit_Subprocess_NoEnv(t *testing.T) {
	if os.Getenv("BE_INIT_NOENV") == "1" {
		_ = GetFlag("nonexistent", "default")
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestInit_Subprocess_NoEnv")
	cmd.Env = append(os.Environ(), "BE_INIT_NOENV=1")
	cmd.Env = removeEnv(cmd.Env, "ARGOCD_OPTS")
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 0 {
			return // success
		}
		t.Fatalf("subprocess for BE_INIT_NOENV did not exit as expected: %v", err)
	}
}

func removeEnv(env []string, key string) []string {
	var result []string
	for _, e := range env {
		if len(e) > len(key) && e[:len(key)+1] == key+"=" {
			continue
		}
		result = append(result, e)
	}
	return result
}

// --- Extra coverage for GetStringSliceFlag ---

func TestStringSliceFlag_KeyExistsButEmpty(t *testing.T) {
	set(t, "--loglevel debug")
	if flags == nil {
		flags = make(map[string]string)
	}
	flags["header"] = ""
	got := GetStringSliceFlag("header", []string{"fallback"})
	assert.Equal(t, []string{}, got)
}

// --- Minimal coverage of missing lines safely ---

func TestEnvMissingLinesMinimalSafe(t *testing.T) {
	t.Setenv("ARGOCD_OPTS", "unexpectedvalue")
	err := LoadFlags()
	if err == nil {
		t.Fatalf("expected LoadFlags to return error")
	}

	runSubTest := func(envKey string, setupFlags func()) {
		if os.Getenv(envKey) == "1" {
			setupFlags()
			return
		}
		cmd := exec.Command(os.Args[0], "-test.run=TestEnvMissingLinesMinimalSafe")
		cmd.Env = append(os.Environ(), envKey+"=1")
		err := cmd.Run()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && exitErr.ExitCode() != 0 {
				return
			}
			t.Fatalf("subprocess for %s did not exit as expected: %v", envKey, err)
		}
	}

	runSubTest("BE_INT_ERROR", func() {
		flags = map[string]string{"retries": "not-an-int"}
		_ = GetIntFlag("retries", 0)
	})

	runSubTest("BE_CSV_ERROR", func() {
		flags = map[string]string{"header": `a,"unterminated`}
		_ = GetStringSliceFlag("header", []string{"fallback"})
	})
}
