package kustomize

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	certutil "github.com/argoproj/argo-cd/v3/util/cert"
	executil "github.com/argoproj/argo-cd/v3/util/exec"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/proxy"

	securejoin "github.com/cyphar/filepath-securejoin"
)

// Image represents a Docker image in the format NAME[:TAG].
type Image = string

type BuildOpts struct {
	KubeVersion string
	APIVersions []string
}

// Kustomize provides wrapper functionality around the `kustomize` command.
type Kustomize interface {
	// Build returns a list of unstructured objects from a `kustomize build` command and extract supported parameters
	Build(opts *v1alpha1.ApplicationSourceKustomize, kustomizeOptions *v1alpha1.KustomizeOptions, envVars *v1alpha1.Env, buildOpts *BuildOpts) ([]*unstructured.Unstructured, []Image, []string, error)
}

// NewKustomizeApp create a new wrapper to run commands on the `kustomize` command-line tool.
func NewKustomizeApp(repoRoot string, path string, creds git.Creds, fromRepo string, binaryPath string, proxy string, noProxy string) Kustomize {
	return &kustomize{
		repoRoot:   repoRoot,
		path:       path,
		creds:      creds,
		repo:       fromRepo,
		binaryPath: binaryPath,
		proxy:      proxy,
		noProxy:    noProxy,
	}
}

type kustomize struct {
	// path to the Git repository root
	repoRoot string
	// path inside the checked out tree
	path string
	// creds structure
	creds git.Creds
	// the Git repository URL where we checked out
	repo string
	// optional kustomize binary path
	binaryPath string
	// HTTP/HTTPS proxy used to access repository
	proxy string
	// NoProxy specifies a list of targets where the proxy isn't used, applies only in cases where the proxy is applied
	noProxy string
}

var KustomizationNames = []string{"kustomization.yaml", "kustomization.yml", "Kustomization"}

// IsKustomization checks if the given file name matches any known kustomization file names.
func IsKustomization(path string) bool {
	for _, kustomization := range KustomizationNames {
		if path == kustomization {
			return true
		}
	}
	return false
}

// findKustomizeFile looks for any known kustomization file in the path
func findKustomizeFile(dir string) string {
	for _, file := range KustomizationNames {
		path := filepath.Join(dir, file)
		if _, err := os.Stat(path); err == nil {
			return file
		}
	}

	return ""
}

func (k *kustomize) getBinaryPath() string {
	if k.binaryPath != "" {
		return k.binaryPath
	}
	return "kustomize"
}

// kustomize v3.8.5 patch release introduced a breaking change in "edit add <label/annotation>" commands:
// https://github.com/kubernetes-sigs/kustomize/commit/b214fa7d5aa51d7c2ae306ec15115bf1c044fed8#diff-0328c59bcd29799e365ff0647653b886f17c8853df008cd54e7981db882c1b36
func mapToEditAddArgs(val map[string]string) []string {
	var args []string
	if getSemverSafe(&kustomize{}).LessThan(semver.MustParse("v3.8.5")) {
		arg := ""
		for labelName, labelValue := range val {
			if arg != "" {
				arg += ","
			}
			arg += fmt.Sprintf("%s:%s", labelName, labelValue)
		}
		args = append(args, arg)
	} else {
		for labelName, labelValue := range val {
			args = append(args, fmt.Sprintf("%s:%s", labelName, labelValue))
		}
	}
	return args
}

// buildContext holds common parameters and state for the build process.
type buildContext struct {
	kustomize     *kustomize
	opts          *v1alpha1.ApplicationSourceKustomize
	kustomizeOpts *v1alpha1.KustomizeOptions
	buildOpts     *BuildOpts
	commands      []string
	userEnv       *v1alpha1.Env
	invocationEnv []string
}

// newBuildContext initializes a new buildContext.
func newBuildContext(k *kustomize, opts *v1alpha1.ApplicationSourceKustomize, kustomizeOpts *v1alpha1.KustomizeOptions, userEnv *v1alpha1.Env, buildOpts *BuildOpts) *buildContext {
	invocationEnv := os.Environ() // Start with system environment

	if userEnv != nil {
		invocationEnv = append(invocationEnv, userEnv.Environ()...)
	}

	return &buildContext{
		kustomize:     k,
		opts:          opts,
		kustomizeOpts: kustomizeOpts,
		userEnv:       userEnv,
		buildOpts:     buildOpts,
		commands:      []string{},
		invocationEnv: invocationEnv,
	}
}

// setupEnvironment prepares the environment variables for Kustomize execution.
func (ctx *buildContext) setupEnvironment() (io.Closer, error) {
	// Defer closer.Close() will be handled by the caller
	closer, credsEnviron, err := ctx.kustomize.creds.Environ()
	if err != nil {
		return nil, err
	}
	ctx.invocationEnv = append(ctx.invocationEnv, credsEnviron...)

	// If we were passed an HTTPS URL, make sure that we also check whether there
	// is a custom CA bundle configured for connecting to the server.
	if ctx.kustomize.repo != "" && git.IsHTTPSURL(ctx.kustomize.repo) {
		parsedURL, err := url.Parse(ctx.kustomize.repo)
		if err != nil {
			log.Warnf("Could not parse URL %s: %v", ctx.kustomize.repo, err)
		} else {
			caPath, err := certutil.GetCertBundlePathForRepository(parsedURL.Host)
			switch {
			case err != nil:
				// Some error while getting CA bundle
				log.Warnf("Could not get CA bundle path for %s: %v", parsedURL.Host, err)
			case caPath == "":
				// No cert configured
				log.Debugf("No caCert found for repo %s", parsedURL.Host)
			default:
				// Make Git use CA bundle
				ctx.invocationEnv = append(ctx.invocationEnv, "GIT_SSL_CAINFO="+caPath)
			}
		}
	}
	return closer, nil
}

// runKustomizeEditCommand runs a kustomize edit command and appends it to the commands list.
func (ctx *buildContext) runKustomizeEditCommand(args ...string) error {
	args = append([]string{"edit"}, args...)
	cmd := exec.Command(ctx.kustomize.getBinaryPath(), args...)
	cmd.Dir = ctx.kustomize.path
	ctx.commands = append(ctx.commands, executil.GetCommandArgsToLog(cmd))
	_, err := executil.Run(cmd)
	return err
}

// applyNamePrefix applies the name prefix if specified.
func (ctx *buildContext) applyNamePrefix() error {
	if ctx.opts.NamePrefix != "" {
		return ctx.runKustomizeEditCommand("set", "nameprefix", "--", ctx.opts.NamePrefix)
	}
	return nil
}

// applyNameSuffix applies the name suffix if specified.
func (ctx *buildContext) applyNameSuffix() error {
	if ctx.opts.NameSuffix != "" {
		return ctx.runKustomizeEditCommand("set", "namesuffix", "--", ctx.opts.NameSuffix)
	}
	return nil
}

// applyImages applies image overrides if specified.
func (ctx *buildContext) applyImages() error {
	// set image postgres=eu.gcr.io/my-project/postgres:latest my-app=my-registry/my-app@sha256:24a0c4b4a4c0eb97a1aabb8e29f18e917d05abfe1b7a7c07857230879ce7d3d3
	// set image node:8.15.0 mysql=mariadb alpine@sha256:24a0c4b4a4c0eb97a1aabb8e29f18e917d05abfe1b7a7c07857230879ce7d3d3
	if len(ctx.opts.Images) > 0 {
		args := []string{"set", "image"}
		for _, image := range ctx.opts.Images {
			// this allows using ${ARGOCD_APP_REVISION}
			envSubstitutedImage := ctx.userEnv.Envsubst(string(image))
			args = append(args, envSubstitutedImage)
		}
		return ctx.runKustomizeEditCommand(args...)
	}
	return nil
}

// applyReplicas applies replica overrides if specified.
func (ctx *buildContext) applyReplicas() error {
	// set replicas my-development=2 my-statefulset=4
	if len(ctx.opts.Replicas) > 0 {
		args := []string{"set", "replicas"}
		for _, replica := range ctx.opts.Replicas {
			count, err := replica.GetIntCount()
			if err != nil {
				return err
			}
			arg := fmt.Sprintf("%s=%d", replica.Name, count)
			args = append(args, arg)
		}
		return ctx.runKustomizeEditCommand(args...)
	}
	return nil
}

// applyCommonLabels applies common labels if specified.
func (ctx *buildContext) applyCommonLabels() error {
	//  edit add label foo:bar
	if len(ctx.opts.CommonLabels) > 0 {
		args := []string{"add", "label"}
		if ctx.opts.ForceCommonLabels {
			args = append(args, "--force")
		}
		if ctx.opts.LabelWithoutSelector {
			args = append(args, "--without-selector")
		}
		if ctx.opts.LabelIncludeTemplates {
			args = append(args, "--include-templates")
		}
		commonLabels := map[string]string{}
		for name, value := range ctx.opts.CommonLabels {
			commonLabels[name] = ctx.userEnv.Envsubst(value)
		}
		return ctx.runKustomizeEditCommand(append(args, mapToEditAddArgs(commonLabels)...)...)
	}
	return nil
}

// applyCommonAnnotations applies common annotations if specified.
func (ctx *buildContext) applyCommonAnnotations() error {
	//  edit add annotation foo:bar
	if len(ctx.opts.CommonAnnotations) > 0 {
		args := []string{"add", "annotation"}
		if ctx.opts.ForceCommonAnnotations {
			args = append(args, "--force")
		}
		var commonAnnotations map[string]string
		if ctx.opts.CommonAnnotationsEnvsubst {
			commonAnnotations = map[string]string{}
			for name, value := range ctx.opts.CommonAnnotations {
				commonAnnotations[name] = ctx.userEnv.Envsubst(value)
			}
		} else {
			commonAnnotations = ctx.opts.CommonAnnotations
		}
		return ctx.runKustomizeEditCommand(append(args, mapToEditAddArgs(commonAnnotations)...)...)
	}
	return nil
}

// applyNamespace applies the namespace if specified.
func (ctx *buildContext) applyNamespace() error {
	if ctx.opts.Namespace != "" {
		return ctx.runKustomizeEditCommand("set", "namespace", "--", ctx.opts.Namespace)
	}
	return nil
}

// applyPatches modifies the kustomization.yaml to add patches.
func (ctx *buildContext) applyPatches() error {
	if len(ctx.opts.Patches) > 0 {
		kustFile := findKustomizeFile(ctx.kustomize.path)
		// If the kustomization file is not found, return early.
		// There is no point reading the kustomization path if it doesn't exist.
		if kustFile == "" {
			return errors.New("kustomization file not found in the path")
		}
		kustomizationPath := filepath.Join(ctx.kustomize.path, kustFile)
		b, err := os.ReadFile(kustomizationPath)
		if err != nil {
			return fmt.Errorf("failed to load kustomization.yaml: %w", err)
		}
		var kustomization any
		err = yaml.Unmarshal(b, &kustomization)
		if err != nil {
			return fmt.Errorf("failed to unmarshal kustomization.yaml: %w", err)
		}
		kMap, ok := kustomization.(map[string]any)
		if !ok {
			return fmt.Errorf("expected kustomization.yaml to be type map[string]any, but got %T", kMap)
		}

		patches, ok := kMap["patches"]
		if ok {
			// The kustomization.yaml already had a patches field, so we need to append to it.
			patchesList, ok := patches.([]any)
			if !ok {
				return fmt.Errorf("expected 'patches' field in kustomization.yaml to be []any, but got %T", patches)
			}
			// Since the patches from the Application manifest are typed, we need to convert them to a type which
			// can be appended to the existing list.
			untypedPatches := make([]any, len(ctx.opts.Patches))
			for i := range ctx.opts.Patches {
				untypedPatches[i] = ctx.opts.Patches[i]
			}
			// Update the kustomization.yaml with the appended patches list.
			patchesList = append(patchesList, untypedPatches...)
			kMap["patches"] = patchesList
		} else {
			kMap["patches"] = ctx.opts.Patches
		}

		updatedKustomization, err := yaml.Marshal(kMap)
		if err != nil {
			return fmt.Errorf("failed to marshal kustomization.yaml after adding patches: %w", err)
		}
		kustomizationFileInfo, err := os.Stat(kustomizationPath)
		if err != nil {
			return fmt.Errorf("failed to stat kustomization.yaml: %w", err)
		}
		err = os.WriteFile(kustomizationPath, updatedKustomization, kustomizationFileInfo.Mode())
		if err != nil {
			return fmt.Errorf("failed to write kustomization.yaml with updated 'patches' field: %w", err)
		}
		ctx.commands = append(ctx.commands, "# kustomization.yaml updated with patches. There is no `kustomize edit` command for adding patches. In order to generate the manifests in your local environment, you will need to copy the patches into kustomization.yaml manually.")
	}
	return nil
}

// applyComponents adds components if specified.
func (ctx *buildContext) applyComponents() error {
	// components only supported in kustomize >= v3.7.0
	// https://github.com/kubernetes-sigs/kustomize/blob/master/examples/components.md
	if len(ctx.opts.Components) > 0 {
		if getSemverSafe(ctx.kustomize).LessThan(semver.MustParse("v3.7.0")) {
			return errors.New("kustomize components require kustomize v3.7.0 and above")
		}

		foundComponents := ctx.opts.Components
		if ctx.opts.IgnoreMissingComponents {
			foundComponents = make([]string, 0)
			for _, c := range ctx.opts.Components {
				resolvedPath, err := securejoin.SecureJoin(ctx.kustomize.path, c)
				if err != nil {
					return fmt.Errorf("kustomize components path failed: %w", err)
				}
				_, err = os.Stat(resolvedPath)
				if err != nil {
					log.Debugf("%s component directory does not exist", resolvedPath)
					continue
				}
				foundComponents = append(foundComponents, c)
			}
		}
		args := []string{"edit", "add", "component"}
		args = append(args, foundComponents...)
		cmd := exec.Command(ctx.kustomize.getBinaryPath(), args...)
		cmd.Dir = ctx.kustomize.path
		cmd.Env = ctx.invocationEnv
		ctx.commands = append(ctx.commands, executil.GetCommandArgsToLog(cmd))
		_, err := executil.Run(cmd)
		return err
	}
	return nil
}

// executeKustomizeBuild runs the final kustomize build command.
func (ctx *buildContext) executeKustomizeBuild() (string, error) {
	var cmd *exec.Cmd
	if ctx.kustomizeOpts != nil && ctx.kustomizeOpts.BuildOptions != "" {
		params := parseKustomizeBuildOptions(ctx.kustomize, ctx.kustomizeOpts.BuildOptions, ctx.buildOpts)
		cmd = exec.Command(ctx.kustomize.getBinaryPath(), params...)
	} else {
		cmd = exec.Command(ctx.kustomize.getBinaryPath(), "build", ctx.kustomize.path)
	}
	cmd.Env = ctx.invocationEnv
	cmd.Env = proxy.UpsertEnv(cmd, ctx.kustomize.proxy, ctx.kustomize.noProxy)
	cmd.Dir = ctx.kustomize.repoRoot
	ctx.commands = append(ctx.commands, executil.GetCommandArgsToLog(cmd))
	out, err := executil.Run(cmd)
	if err != nil {
		return "", err
	}
	return out, nil
}

// processBuildOutput parses the YAML output and redacts commands.
func (ctx *buildContext) processBuildOutput(output string) ([]*unstructured.Unstructured, []Image, []string, error) {
	objs, err := kube.SplitYAML([]byte(output))
	if err != nil {
		return nil, nil, nil, err
	}

	redactedCommands := make([]string, len(ctx.commands))
	for i, c := range ctx.commands {
		redactedCommands[i] = strings.ReplaceAll(c, ctx.kustomize.repoRoot, ".")
	}

	return objs, getImageParameters(objs), redactedCommands, nil
}

func (k *kustomize) Build(opts *v1alpha1.ApplicationSourceKustomize, kustomizeOpts *v1alpha1.KustomizeOptions, envVars *v1alpha1.Env, buildOpts *BuildOpts) ([]*unstructured.Unstructured, []Image, []string, error) {
	ctx := newBuildContext(k, opts, kustomizeOpts, envVars, buildOpts)

	closer, err := ctx.setupEnvironment()
	if err != nil {
		return nil, nil, nil, err
	}
	defer func() { _ = closer.Close() }()

	if ctx.opts != nil {
		if err := ctx.applyNamePrefix(); err != nil {
			return nil, nil, nil, err
		}
		if err := ctx.applyNameSuffix(); err != nil {
			return nil, nil, nil, err
		}
		if err := ctx.applyImages(); err != nil {
			return nil, nil, nil, err
		}
		if err := ctx.applyReplicas(); err != nil {
			return nil, nil, nil, err
		}
		if err := ctx.applyCommonLabels(); err != nil {
			return nil, nil, nil, err
		}
		if err := ctx.applyCommonAnnotations(); err != nil {
			return nil, nil, nil, err
		}
		if err := ctx.applyNamespace(); err != nil {
			return nil, nil, nil, err
		}
		if err := ctx.applyPatches(); err != nil {
			return nil, nil, nil, err
		}
		if err := ctx.applyComponents(); err != nil {
			return nil, nil, nil, err
		}
	}

	output, err := ctx.executeKustomizeBuild()
	if err != nil {
		return nil, nil, nil, err
	}

	return ctx.processBuildOutput(output)
}

func parseKustomizeBuildOptions(k *kustomize, buildOptions string, buildOpts *BuildOpts) []string {
	buildOptsParams := append([]string{"build", k.path}, strings.Fields(buildOptions)...)

	if buildOpts != nil && !getSemverSafe(k).LessThan(semver.MustParse("v5.3.0")) && isHelmEnabled(buildOptions) {
		if buildOpts.KubeVersion != "" {
			buildOptsParams = append(buildOptsParams, "--helm-kube-version", buildOpts.KubeVersion)
		}
		for _, v := range buildOpts.APIVersions {
			buildOptsParams = append(buildOptsParams, "--helm-api-versions", v)
		}
	}

	return buildOptsParams
}

func isHelmEnabled(buildOptions string) bool {
	return strings.Contains(buildOptions, "--enable-helm")
}

// semver/v3 doesn't export the regexp anymore, so shamelessly copied it over to
// here.
// https://github.com/Masterminds/semver/blob/49c09bfed6adcffa16482ddc5e5588cffff9883a/version.go#L42
const semVerRegex string = `v?([0-9]+)(\.[0-9]+)?(\.[0-9]+)?` +
	`(-([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?` +
	`(\+([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?`

var (
	unknownVersion = semver.MustParse("v99.99.99")
	semverRegex    = regexp.MustCompile(semVerRegex)
	semVer         *semver.Version
	semVerLock     sync.Mutex
)

// getSemver returns parsed kustomize version
func getSemver(k *kustomize) (*semver.Version, error) {
	verStr, err := versionWithBinaryPath(k)
	if err != nil {
		return nil, err
	}

	semverMatches := semverRegex.FindStringSubmatch(verStr)
	if len(semverMatches) == 0 {
		return nil, fmt.Errorf("expected string that includes semver formatted version but got: '%s'", verStr)
	}

	return semver.NewVersion(semverMatches[0])
}

// getSemverSafe returns parsed kustomize version;
// if version cannot be parsed assumes that "kustomize version" output format changed again
// and fallback to latest ( v99.99.99 )
func getSemverSafe(k *kustomize) *semver.Version {
	if semVer == nil {
		semVerLock.Lock()
		defer semVerLock.Unlock()

		if ver, err := getSemver(k); err != nil {
			semVer = unknownVersion
			log.Warnf("Failed to parse kustomize version: %v", err)
		} else {
			semVer = ver
		}
	}
	return semVer
}

func Version() (string, error) {
	return versionWithBinaryPath(&kustomize{})
}

func versionWithBinaryPath(k *kustomize) (string, error) {
	executable := k.getBinaryPath()
	cmd := exec.Command(executable, "version", "--short")
	// example version output:
	// short: "{kustomize/v3.8.1  2020-07-16T00:58:46Z  }"
	version, err := executil.Run(cmd)
	if err != nil {
		return "", fmt.Errorf("could not get kustomize version: %w", err)
	}
	version = strings.TrimSpace(version)
	// trim the curly braces
	version = strings.TrimPrefix(version, "{")
	version = strings.TrimSuffix(version, "}")
	version = strings.TrimSpace(version)

	// remove double space in middle
	version = strings.ReplaceAll(version, "  ", " ")

	// remove extra 'kustomize/' before version
	version = strings.TrimPrefix(version, "kustomize/")
	return version, nil
}

func getImageParameters(objs []*unstructured.Unstructured) []Image {
	var images []Image
	for _, obj := range objs {
		images = append(images, getImages(obj.Object)...)
	}
	sort.Slice(images, func(i, j int) bool {
		return i < j
	})
	return images
}

func getImages(object map[string]any) []Image {
	var images []Image
	for k, v := range object {
		if array, ok := v.([]any); ok {
			if k == "containers" || k == "initContainers" {
				for _, obj := range array {
					if mapObj, isMapObj := obj.(map[string]any); isMapObj {
						if image, hasImage := mapObj["image"]; hasImage {
							images = append(images, fmt.Sprintf("%s", image))
						}
					}
				}
			} else {
				for i := range array {
					if mapObj, isMapObj := array[i].(map[string]any); isMapObj {
						images = append(images, getImages(mapObj)...)
					}
				}
			}
		} else if objMap, ok := v.(map[string]any); ok {
			images = append(images, getImages(objMap)...)
		}
	}
	return images
}
