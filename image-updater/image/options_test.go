package image

import (
	"fmt"
	"regexp"
	"runtime"
	"testing"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/image-updater/options"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetHelmOptions(t *testing.T) {
	t.Run("Get Helm parameter for configured application", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.HelmParamImageNameAnnotation, "dummy"): "release.name",
			fmt.Sprintf(common.HelmParamImageTagAnnotation, "dummy"):  "release.tag",
			fmt.Sprintf(common.HelmParamImageSpecAnnotation, "dummy"): "release.image",
		}

		img := NewFromIdentifier("dummy=foo/bar:1.12")
		paramName := img.GetParameterHelmImageName(annotations)
		paramTag := img.GetParameterHelmImageTag(annotations)
		paramSpec := img.GetParameterHelmImageSpec(annotations)
		assert.Equal(t, "release.name", paramName)
		assert.Equal(t, "release.tag", paramTag)
		assert.Equal(t, "release.image", paramSpec)
	})

	t.Run("Get Helm parameter for non-configured application", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.HelmParamImageNameAnnotation, "dummy"): "release.name",
			fmt.Sprintf(common.HelmParamImageTagAnnotation, "dummy"):  "release.tag",
			fmt.Sprintf(common.HelmParamImageSpecAnnotation, "dummy"): "release.image",
		}

		img := NewFromIdentifier("foo=foo/bar:1.12")
		paramName := img.GetParameterHelmImageName(annotations)
		paramTag := img.GetParameterHelmImageTag(annotations)
		paramSpec := img.GetParameterHelmImageSpec(annotations)
		assert.Equal(t, "", paramName)
		assert.Equal(t, "", paramTag)
		assert.Equal(t, "", paramSpec)
	})

	t.Run("Get Helm parameter for configured application with normalized name", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.HelmParamImageNameAnnotation, "foo_dummy"): "release.name",
			fmt.Sprintf(common.HelmParamImageTagAnnotation, "foo_dummy"):  "release.tag",
			fmt.Sprintf(common.HelmParamImageSpecAnnotation, "foo_dummy"): "release.image",
		}

		img := NewFromIdentifier("foo/dummy=foo/bar:1.12")
		paramName := img.GetParameterHelmImageName(annotations)
		paramTag := img.GetParameterHelmImageTag(annotations)
		paramSpec := img.GetParameterHelmImageSpec(annotations)
		assert.Equal(t, "release.name", paramName)
		assert.Equal(t, "release.tag", paramTag)
		assert.Equal(t, "release.image", paramSpec)
	})
}

func Test_GetKustomizeOptions(t *testing.T) {
	t.Run("Get Helm parameter for configured application", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.KustomizeApplicationNameAnnotation, "dummy"): "argoproj/argo-cd",
		}

		img := NewFromIdentifier("dummy=foo/bar:1.12")
		paramName := img.GetParameterKustomizeImageName(annotations)
		assert.Equal(t, "argoproj/argo-cd", paramName)
	})
}

func Test_GetSortOption(t *testing.T) {
	t.Run("Get update strategy semver for configured application", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.UpdateStrategyAnnotation, "dummy"): "semver",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		sortMode := img.GetParameterUpdateStrategy(annotations)
		assert.Equal(t, StrategySemVer, sortMode)
	})

	t.Run("Use update strategy newest-build for configured application", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.UpdateStrategyAnnotation, "dummy"): "newest-build",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		sortMode := img.GetParameterUpdateStrategy(annotations)
		assert.Equal(t, StrategyNewestBuild, sortMode)
	})

	t.Run("Get update strategy date for configured application", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.UpdateStrategyAnnotation, "dummy"): "latest",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		sortMode := img.GetParameterUpdateStrategy(annotations)
		assert.Equal(t, StrategyNewestBuild, sortMode)
	})

	t.Run("Get update strategy name for configured application", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.UpdateStrategyAnnotation, "dummy"): "name",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		sortMode := img.GetParameterUpdateStrategy(annotations)
		assert.Equal(t, StrategyAlphabetical, sortMode)
	})

	t.Run("Use update strategy alphabetical for configured application", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.UpdateStrategyAnnotation, "dummy"): "alphabetical",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		sortMode := img.GetParameterUpdateStrategy(annotations)
		assert.Equal(t, StrategyAlphabetical, sortMode)
	})

	t.Run("Get update strategy option configured application because of invalid option", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.UpdateStrategyAnnotation, "dummy"): "invalid",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		sortMode := img.GetParameterUpdateStrategy(annotations)
		assert.Equal(t, StrategySemVer, sortMode)
	})

	t.Run("Get update strategy option configured application because of option not set", func(t *testing.T) {
		annotations := map[string]string{}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		sortMode := img.GetParameterUpdateStrategy(annotations)
		assert.Equal(t, StrategySemVer, sortMode)
	})

	t.Run("Prefer update strategy option from image-specific annotation", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.UpdateStrategyAnnotation, "dummy"): "alphabetical",
			common.ApplicationWideUpdateStrategyAnnotation:        "newest-build",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		sortMode := img.GetParameterUpdateStrategy(annotations)
		assert.Equal(t, StrategyAlphabetical, sortMode)
	})

	t.Run("Get update strategy option from application-wide annotation", func(t *testing.T) {
		annotations := map[string]string{
			common.ApplicationWideUpdateStrategyAnnotation: "newest-build",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		sortMode := img.GetParameterUpdateStrategy(annotations)
		assert.Equal(t, StrategyNewestBuild, sortMode)
	})
}

func Test_GetMatchOption(t *testing.T) {
	t.Run("Get regexp match option for configured application", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.AllowTagsOptionAnnotation, "dummy"): "regexp:a-z",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		matchFunc, matchArgs := img.GetParameterMatch(annotations)
		require.NotNil(t, matchFunc)
		require.NotNil(t, matchArgs)
		assert.IsType(t, &regexp.Regexp{}, matchArgs)
	})

	t.Run("Get regexp match option for configured application with invalid expression", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.AllowTagsOptionAnnotation, "dummy"): `regexp:/foo\`,
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		matchFunc, matchArgs := img.GetParameterMatch(annotations)
		require.NotNil(t, matchFunc)
		require.Nil(t, matchArgs)
	})

	t.Run("Get invalid match option for configured application", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.AllowTagsOptionAnnotation, "dummy"): "invalid",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		matchFunc, matchArgs := img.GetParameterMatch(annotations)
		require.NotNil(t, matchFunc)
		require.Equal(t, false, matchFunc("", nil))
		assert.Nil(t, matchArgs)
	})

	t.Run("Prefer match option from image-specific annotation", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.AllowTagsOptionAnnotation, "dummy"): "regexp:^[0-9]",
			common.ApplicationWideAllowTagsOptionAnnotation:        "regexp:^v",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		matchFunc, matchArgs := img.GetParameterMatch(annotations)
		require.NotNil(t, matchFunc)
		require.NotNil(t, matchArgs)
		assert.IsType(t, &regexp.Regexp{}, matchArgs)
		assert.True(t, matchFunc("0.0.1", matchArgs))
		assert.False(t, matchFunc("v0.0.1", matchArgs))
	})

	t.Run("Get match option from application-wide annotation", func(t *testing.T) {
		annotations := map[string]string{
			common.ApplicationWideAllowTagsOptionAnnotation: "regexp:^v",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		matchFunc, matchArgs := img.GetParameterMatch(annotations)
		require.NotNil(t, matchFunc)
		require.NotNil(t, matchArgs)
		assert.IsType(t, &regexp.Regexp{}, matchArgs)
		assert.False(t, matchFunc("0.0.1", matchArgs))
		assert.True(t, matchFunc("v0.0.1", matchArgs))
	})
}

func Test_GetSecretOption(t *testing.T) {
	t.Run("Get cred source from annotation", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.PullSecretAnnotation, "dummy"): "pullsecret:foo/bar",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		credSrc := img.GetParameterPullSecret(annotations)
		require.NotNil(t, credSrc)
		assert.Equal(t, CredentialSourcePullSecret, credSrc.Type)
		assert.Equal(t, "foo", credSrc.SecretNamespace)
		assert.Equal(t, "bar", credSrc.SecretName)
		assert.Equal(t, ".dockerconfigjson", credSrc.SecretField)
	})

	t.Run("Invalid reference in annotation", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.PullSecretAnnotation, "dummy"): "foo/bar",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		credSrc := img.GetParameterPullSecret(annotations)
		require.Nil(t, credSrc)
	})

	t.Run("Prefer cred source from image-specific annotation", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.PullSecretAnnotation, "dummy"): "pullsecret:image/specific",
			common.ApplicationWidePullSecretAnnotation:        "pullsecret:app/wide",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		credSrc := img.GetParameterPullSecret(annotations)
		require.NotNil(t, credSrc)
		assert.Equal(t, CredentialSourcePullSecret, credSrc.Type)
		assert.Equal(t, "image", credSrc.SecretNamespace)
		assert.Equal(t, "specific", credSrc.SecretName)
		assert.Equal(t, ".dockerconfigjson", credSrc.SecretField)
	})

	t.Run("Get cred source from application-wide annotation", func(t *testing.T) {
		annotations := map[string]string{
			common.ApplicationWidePullSecretAnnotation: "pullsecret:app/wide",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		credSrc := img.GetParameterPullSecret(annotations)
		require.NotNil(t, credSrc)
		assert.Equal(t, CredentialSourcePullSecret, credSrc.Type)
		assert.Equal(t, "app", credSrc.SecretNamespace)
		assert.Equal(t, "wide", credSrc.SecretName)
		assert.Equal(t, ".dockerconfigjson", credSrc.SecretField)
	})
}

func Test_GetIgnoreTags(t *testing.T) {
	t.Run("Get list of tags to ignore from image-specific annotation", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.IgnoreTagsOptionAnnotation, "dummy"): "tag1, ,tag2,  tag3  , tag4",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		tags := img.GetParameterIgnoreTags(annotations)
		require.Len(t, tags, 4)
		assert.Equal(t, "tag1", tags[0])
		assert.Equal(t, "tag2", tags[1])
		assert.Equal(t, "tag3", tags[2])
		assert.Equal(t, "tag4", tags[3])
	})

	t.Run("Prefer list of tags to ignore from image-specific annotation", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.IgnoreTagsOptionAnnotation, "dummy"): "tag1, tag2",
			common.ApplicationWideIgnoreTagsOptionAnnotation:        "tag3, tag4",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		tags := img.GetParameterIgnoreTags(annotations)
		require.Len(t, tags, 2)
		assert.Equal(t, "tag1", tags[0])
		assert.Equal(t, "tag2", tags[1])
	})

	t.Run("Get list of tags to ignore from application-wide annotation", func(t *testing.T) {
		annotations := map[string]string{
			common.ApplicationWideIgnoreTagsOptionAnnotation: "tag3, tag4",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		tags := img.GetParameterIgnoreTags(annotations)
		require.Len(t, tags, 2)
		assert.Equal(t, "tag3", tags[0])
		assert.Equal(t, "tag4", tags[1])
	})
}

func Test_HasForceUpdateOptionAnnotation(t *testing.T) {
	t.Run("Get force-update option from image-specific annotation", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.ForceUpdateOptionAnnotation, "dummy"): "true",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		forceUpdate := img.HasForceUpdateOptionAnnotation(annotations)
		assert.True(t, forceUpdate)
	})

	t.Run("Prefer force-update option from image-specific annotation", func(t *testing.T) {
		annotations := map[string]string{
			fmt.Sprintf(common.ForceUpdateOptionAnnotation, "dummy"): "true",
			common.ApplicationWideForceUpdateOptionAnnotation:        "false",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		forceUpdate := img.HasForceUpdateOptionAnnotation(annotations)
		assert.True(t, forceUpdate)
	})

	t.Run("Get force-update option from application-wide annotation", func(t *testing.T) {
		annotations := map[string]string{
			common.ApplicationWideForceUpdateOptionAnnotation: "false",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		forceUpdate := img.HasForceUpdateOptionAnnotation(annotations)
		assert.False(t, forceUpdate)
	})
}

func Test_GetPlatformOptions(t *testing.T) {
	t.Run("Empty platform options with restriction", func(t *testing.T) {
		annotations := map[string]string{}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		opts := img.GetPlatformOptions(annotations, false)
		os := runtime.GOOS
		arch := runtime.GOARCH
		assert.True(t, opts.WantsPlatform(os, arch, ""))
		assert.False(t, opts.WantsPlatform(os, arch, "invalid"))
	})
	t.Run("Empty platform options without restriction", func(t *testing.T) {
		annotations := map[string]string{}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		opts := img.GetPlatformOptions(annotations, true)
		os := runtime.GOOS
		arch := runtime.GOARCH
		assert.True(t, opts.WantsPlatform(os, arch, ""))
		assert.True(t, opts.WantsPlatform(os, arch, "invalid"))
		assert.True(t, opts.WantsPlatform("windows", "amd64", ""))
	})
	t.Run("Single platform without variant requested", func(t *testing.T) {
		os := "linux"
		arch := "arm64"
		variant := ""
		annotations := map[string]string{
			fmt.Sprintf(common.PlatformsAnnotation, "dummy"): options.PlatformKey(os, arch, variant),
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		opts := img.GetPlatformOptions(annotations, false)
		assert.True(t, opts.WantsPlatform(os, arch, variant))
		assert.False(t, opts.WantsPlatform(os, arch, "invalid"))
	})
	t.Run("Single platform with variant requested", func(t *testing.T) {
		os := "linux"
		arch := "arm"
		variant := "v6"
		annotations := map[string]string{
			fmt.Sprintf(common.PlatformsAnnotation, "dummy"): options.PlatformKey(os, arch, variant),
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		opts := img.GetPlatformOptions(annotations, false)
		assert.True(t, opts.WantsPlatform(os, arch, variant))
		assert.False(t, opts.WantsPlatform(os, arch, ""))
		assert.False(t, opts.WantsPlatform(runtime.GOOS, runtime.GOARCH, ""))
		assert.False(t, opts.WantsPlatform(runtime.GOOS, runtime.GOARCH, variant))
	})
	t.Run("Multiple platforms requested", func(t *testing.T) {
		os := "linux"
		arch := "arm"
		variant := "v6"
		annotations := map[string]string{
			fmt.Sprintf(common.PlatformsAnnotation, "dummy"): options.PlatformKey(os, arch, variant) + ", " + options.PlatformKey(runtime.GOOS, runtime.GOARCH, ""),
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		opts := img.GetPlatformOptions(annotations, false)
		assert.True(t, opts.WantsPlatform(os, arch, variant))
		assert.True(t, opts.WantsPlatform(runtime.GOOS, runtime.GOARCH, ""))
		assert.False(t, opts.WantsPlatform(os, arch, ""))
		assert.False(t, opts.WantsPlatform(runtime.GOOS, runtime.GOARCH, variant))
	})
	t.Run("Invalid platform requested", func(t *testing.T) {
		os := "linux"
		arch := "arm"
		variant := "v6"
		annotations := map[string]string{
			fmt.Sprintf(common.PlatformsAnnotation, "dummy"): "invalid",
		}
		img := NewFromIdentifier("dummy=foo/bar:1.12")
		opts := img.GetPlatformOptions(annotations, false)
		assert.False(t, opts.WantsPlatform(os, arch, variant))
		assert.False(t, opts.WantsPlatform(runtime.GOOS, runtime.GOARCH, ""))
		assert.False(t, opts.WantsPlatform(os, arch, ""))
		assert.False(t, opts.WantsPlatform(runtime.GOOS, runtime.GOARCH, variant))
	})
}
