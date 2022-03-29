package util

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/config"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/text/label"
)

type AppOptions struct {
	repoURL                         string
	appPath                         string
	chart                           string
	env                             string
	revision                        string
	revisionHistoryLimit            int
	destName                        string
	destServer                      string
	destNamespace                   string
	Parameters                      []string
	valuesFiles                     []string
	ignoreMissingValueFiles         bool
	values                          string
	releaseName                     string
	helmSets                        []string
	helmSetStrings                  []string
	helmSetFiles                    []string
	helmVersion                     string
	helmPassCredentials             bool
	helmSkipCrds                    bool
	project                         string
	syncPolicy                      string
	syncOptions                     []string
	autoPrune                       bool
	selfHeal                        bool
	allowEmpty                      bool
	namePrefix                      string
	nameSuffix                      string
	directoryRecurse                bool
	configManagementPlugin          string
	jsonnetTlaStr                   []string
	jsonnetTlaCode                  []string
	jsonnetExtVarStr                []string
	jsonnetExtVarCode               []string
	jsonnetLibs                     []string
	kustomizeImages                 []string
	kustomizeVersion                string
	kustomizeCommonLabels           []string
	kustomizeCommonAnnotations      []string
	kustomizeForceCommonLabels      bool
	kustomizeForceCommonAnnotations bool
	pluginEnvs                      []string
	Validate                        bool
	directoryExclude                string
	directoryInclude                string
	retryLimit                      int64
	retryBackoffDuration            time.Duration
	retryBackoffMaxDuration         time.Duration
	retryBackoffFactor              int64
}

func AddAppFlags(command *cobra.Command, opts *AppOptions) {
	command.Flags().StringVar(&opts.repoURL, "repo", "", "Repository URL, ignored if a file is set")
	command.Flags().StringVar(&opts.appPath, "path", "", "Path in repository to the app directory, ignored if a file is set")
	command.Flags().StringVar(&opts.chart, "helm-chart", "", "Helm Chart name")
	command.Flags().StringVar(&opts.env, "env", "", "Application environment to monitor")
	command.Flags().StringVar(&opts.revision, "revision", "", "The tracking source branch, tag, commit or Helm chart version the application will sync to")
	command.Flags().IntVar(&opts.revisionHistoryLimit, "revision-history-limit", argoappv1.RevisionHistoryLimit, "How many items to keep in revision history")
	command.Flags().StringVar(&opts.destServer, "dest-server", "", "K8s cluster URL (e.g. https://kubernetes.default.svc)")
	command.Flags().StringVar(&opts.destName, "dest-name", "", "K8s cluster Name (e.g. minikube)")
	command.Flags().StringVar(&opts.destNamespace, "dest-namespace", "", "K8s target namespace")
	command.Flags().StringArrayVarP(&opts.Parameters, "parameter", "p", []string{}, "set a parameter override (e.g. -p guestbook=image=example/guestbook:latest)")
	command.Flags().StringArrayVar(&opts.valuesFiles, "values", []string{}, "Helm values file(s) to use")
	command.Flags().BoolVar(&opts.ignoreMissingValueFiles, "ignore-missing-value-files", false, "Ignore locally missing valueFiles when setting helm template --values")
	command.Flags().StringVar(&opts.values, "values-literal-file", "", "Filename or URL to import as a literal Helm values block")
	command.Flags().StringVar(&opts.releaseName, "release-name", "", "Helm release-name")
	command.Flags().StringVar(&opts.helmVersion, "helm-version", "", "Helm version")
	command.Flags().BoolVar(&opts.helmPassCredentials, "helm-pass-credentials", false, "Pass credentials to all domain")
	command.Flags().StringArrayVar(&opts.helmSets, "helm-set", []string{}, "Helm set values on the command line (can be repeated to set several values: --helm-set key1=val1 --helm-set key2=val2)")
	command.Flags().StringArrayVar(&opts.helmSetStrings, "helm-set-string", []string{}, "Helm set STRING values on the command line (can be repeated to set several values: --helm-set-string key1=val1 --helm-set-string key2=val2)")
	command.Flags().StringArrayVar(&opts.helmSetFiles, "helm-set-file", []string{}, "Helm set values from respective files specified via the command line (can be repeated to set several values: --helm-set-file key1=path1 --helm-set-file key2=path2)")
	command.Flags().BoolVar(&opts.helmSkipCrds, "helm-skip-crds", false, "Skip helm crd installation step")
	command.Flags().StringVar(&opts.project, "project", "", "Application project name")
	command.Flags().StringVar(&opts.syncPolicy, "sync-policy", "", "Set the sync policy (one of: none, automated (aliases of automated: auto, automatic))")
	command.Flags().StringArrayVar(&opts.syncOptions, "sync-option", []string{}, "Add or remove a sync option, e.g add `Prune=false`. Remove using `!` prefix, e.g. `!Prune=false`")
	command.Flags().BoolVar(&opts.autoPrune, "auto-prune", false, "Set automatic pruning when sync is automated")
	command.Flags().BoolVar(&opts.selfHeal, "self-heal", false, "Set self healing when sync is automated")
	command.Flags().BoolVar(&opts.allowEmpty, "allow-empty", false, "Set allow zero live resources when sync is automated")
	command.Flags().StringVar(&opts.namePrefix, "nameprefix", "", "Kustomize nameprefix")
	command.Flags().StringVar(&opts.nameSuffix, "namesuffix", "", "Kustomize namesuffix")
	command.Flags().StringVar(&opts.kustomizeVersion, "kustomize-version", "", "Kustomize version")
	command.Flags().BoolVar(&opts.directoryRecurse, "directory-recurse", false, "Recurse directory")
	command.Flags().StringVar(&opts.configManagementPlugin, "config-management-plugin", "", "Config management plugin name")
	command.Flags().StringArrayVar(&opts.jsonnetTlaStr, "jsonnet-tla-str", []string{}, "Jsonnet top level string arguments")
	command.Flags().StringArrayVar(&opts.jsonnetTlaCode, "jsonnet-tla-code", []string{}, "Jsonnet top level code arguments")
	command.Flags().StringArrayVar(&opts.jsonnetExtVarStr, "jsonnet-ext-var-str", []string{}, "Jsonnet string ext var")
	command.Flags().StringArrayVar(&opts.jsonnetExtVarCode, "jsonnet-ext-var-code", []string{}, "Jsonnet ext var")
	command.Flags().StringArrayVar(&opts.jsonnetLibs, "jsonnet-libs", []string{}, "Additional jsonnet libs (prefixed by repoRoot)")
	command.Flags().StringArrayVar(&opts.kustomizeImages, "kustomize-image", []string{}, "Kustomize images (e.g. --kustomize-image node:8.15.0 --kustomize-image mysql=mariadb,alpine@sha256:24a0c4b4a4c0eb97a1aabb8e29f18e917d05abfe1b7a7c07857230879ce7d3d)")
	command.Flags().StringArrayVar(&opts.pluginEnvs, "plugin-env", []string{}, "Additional plugin envs")
	command.Flags().BoolVar(&opts.Validate, "validate", true, "Validation of repo and cluster")
	command.Flags().StringArrayVar(&opts.kustomizeCommonLabels, "kustomize-common-label", []string{}, "Set common labels in Kustomize")
	command.Flags().StringArrayVar(&opts.kustomizeCommonAnnotations, "kustomize-common-annotation", []string{}, "Set common labels in Kustomize")
	command.Flags().BoolVar(&opts.kustomizeForceCommonLabels, "kustomize-force-common-label", false, "Force common labels in Kustomize")
	command.Flags().BoolVar(&opts.kustomizeForceCommonAnnotations, "kustomize-force-common-annotation", false, "Force common annotations in Kustomize")
	command.Flags().StringVar(&opts.directoryExclude, "directory-exclude", "", "Set glob expression used to exclude files from application source path")
	command.Flags().StringVar(&opts.directoryInclude, "directory-include", "", "Set glob expression used to include files from application source path")
	command.Flags().Int64Var(&opts.retryLimit, "sync-retry-limit", 0, "Max number of allowed sync retries")
	command.Flags().DurationVar(&opts.retryBackoffDuration, "sync-retry-backoff-duration", argoappv1.DefaultSyncRetryDuration, "Sync retry backoff base duration. Input needs to be a duration (e.g. 2m, 1h)")
	command.Flags().DurationVar(&opts.retryBackoffMaxDuration, "sync-retry-backoff-max-duration", argoappv1.DefaultSyncRetryMaxDuration, "Max sync retry backoff duration. Input needs to be a duration (e.g. 2m, 1h)")
	command.Flags().Int64Var(&opts.retryBackoffFactor, "sync-retry-backoff-factor", argoappv1.DefaultSyncRetryFactor, "Factor multiplies the base duration after each failed sync retry")
}

func SetAppSpecOptions(flags *pflag.FlagSet, spec *argoappv1.ApplicationSpec, appOpts *AppOptions) int {
	visited := 0
	if flags == nil {
		return visited
	}
	flags.Visit(func(f *pflag.Flag) {
		visited++
		switch f.Name {
		case "repo":
			spec.Source.RepoURL = appOpts.repoURL
		case "path":
			spec.Source.Path = appOpts.appPath
		case "helm-chart":
			spec.Source.Chart = appOpts.chart
		case "revision":
			spec.Source.TargetRevision = appOpts.revision
		case "revision-history-limit":
			i := int64(appOpts.revisionHistoryLimit)
			spec.RevisionHistoryLimit = &i
		case "values":
			setHelmOpt(&spec.Source, helmOpts{valueFiles: appOpts.valuesFiles})
		case "ignore-missing-value-files":
			setHelmOpt(&spec.Source, helmOpts{ignoreMissingValueFiles: appOpts.ignoreMissingValueFiles})
		case "values-literal-file":
			var data []byte

			// read uri
			parsedURL, err := url.ParseRequestURI(appOpts.values)
			if err != nil || !(parsedURL.Scheme == "http" || parsedURL.Scheme == "https") {
				data, err = ioutil.ReadFile(appOpts.values)
			} else {
				data, err = config.ReadRemoteFile(appOpts.values)
			}
			errors.CheckError(err)
			setHelmOpt(&spec.Source, helmOpts{values: string(data)})
		case "release-name":
			setHelmOpt(&spec.Source, helmOpts{releaseName: appOpts.releaseName})
		case "helm-version":
			setHelmOpt(&spec.Source, helmOpts{version: appOpts.helmVersion})
		case "helm-pass-credentials":
			setHelmOpt(&spec.Source, helmOpts{passCredentials: appOpts.helmPassCredentials})
		case "helm-set":
			setHelmOpt(&spec.Source, helmOpts{helmSets: appOpts.helmSets})
		case "helm-set-string":
			setHelmOpt(&spec.Source, helmOpts{helmSetStrings: appOpts.helmSetStrings})
		case "helm-set-file":
			setHelmOpt(&spec.Source, helmOpts{helmSetFiles: appOpts.helmSetFiles})
		case "helm-skip-crds":
			setHelmOpt(&spec.Source, helmOpts{skipCrds: appOpts.helmSkipCrds})
		case "directory-recurse":
			if spec.Source.Directory != nil {
				spec.Source.Directory.Recurse = appOpts.directoryRecurse
			} else {
				spec.Source.Directory = &argoappv1.ApplicationSourceDirectory{Recurse: appOpts.directoryRecurse}
			}
		case "directory-exclude":
			if spec.Source.Directory != nil {
				spec.Source.Directory.Exclude = appOpts.directoryExclude
			} else {
				spec.Source.Directory = &argoappv1.ApplicationSourceDirectory{Exclude: appOpts.directoryExclude}
			}
		case "directory-include":
			if spec.Source.Directory != nil {
				spec.Source.Directory.Include = appOpts.directoryInclude
			} else {
				spec.Source.Directory = &argoappv1.ApplicationSourceDirectory{Include: appOpts.directoryInclude}
			}
		case "config-management-plugin":
			spec.Source.Plugin = &argoappv1.ApplicationSourcePlugin{Name: appOpts.configManagementPlugin}
		case "dest-name":
			spec.Destination.Name = appOpts.destName
		case "dest-server":
			spec.Destination.Server = appOpts.destServer
		case "dest-namespace":
			spec.Destination.Namespace = appOpts.destNamespace
		case "project":
			spec.Project = appOpts.project
		case "nameprefix":
			setKustomizeOpt(&spec.Source, kustomizeOpts{namePrefix: appOpts.namePrefix})
		case "namesuffix":
			setKustomizeOpt(&spec.Source, kustomizeOpts{nameSuffix: appOpts.nameSuffix})
		case "kustomize-image":
			setKustomizeOpt(&spec.Source, kustomizeOpts{images: appOpts.kustomizeImages})
		case "kustomize-version":
			setKustomizeOpt(&spec.Source, kustomizeOpts{version: appOpts.kustomizeVersion})
		case "kustomize-common-label":
			parsedLabels, err := label.Parse(appOpts.kustomizeCommonLabels)
			errors.CheckError(err)
			setKustomizeOpt(&spec.Source, kustomizeOpts{commonLabels: parsedLabels})
		case "kustomize-common-annotation":
			parsedAnnotations, err := label.Parse(appOpts.kustomizeCommonAnnotations)
			errors.CheckError(err)
			setKustomizeOpt(&spec.Source, kustomizeOpts{commonAnnotations: parsedAnnotations})
		case "kustomize-force-common-label":
			setKustomizeOpt(&spec.Source, kustomizeOpts{forceCommonLabels: appOpts.kustomizeForceCommonLabels})
		case "kustomize-force-common-annotation":
			setKustomizeOpt(&spec.Source, kustomizeOpts{forceCommonAnnotations: appOpts.kustomizeForceCommonAnnotations})
		case "jsonnet-tla-str":
			setJsonnetOpt(&spec.Source, appOpts.jsonnetTlaStr, false)
		case "jsonnet-tla-code":
			setJsonnetOpt(&spec.Source, appOpts.jsonnetTlaCode, true)
		case "jsonnet-ext-var-str":
			setJsonnetOptExtVar(&spec.Source, appOpts.jsonnetExtVarStr, false)
		case "jsonnet-ext-var-code":
			setJsonnetOptExtVar(&spec.Source, appOpts.jsonnetExtVarCode, true)
		case "jsonnet-libs":
			setJsonnetOptLibs(&spec.Source, appOpts.jsonnetLibs)
		case "plugin-env":
			setPluginOptEnvs(&spec.Source, appOpts.pluginEnvs)
		case "sync-policy":
			switch appOpts.syncPolicy {
			case "none":
				if spec.SyncPolicy != nil {
					spec.SyncPolicy.Automated = nil
				}
				if spec.SyncPolicy.IsZero() {
					spec.SyncPolicy = nil
				}
			case "automated", "automatic", "auto":
				if spec.SyncPolicy == nil {
					spec.SyncPolicy = &argoappv1.SyncPolicy{}
				}
				spec.SyncPolicy.Automated = &argoappv1.SyncPolicyAutomated{}
			default:
				log.Fatalf("Invalid sync-policy: %s", appOpts.syncPolicy)
			}
		case "sync-option":
			if spec.SyncPolicy == nil {
				spec.SyncPolicy = &argoappv1.SyncPolicy{}
			}
			for _, option := range appOpts.syncOptions {
				// `!` means remove the option
				if strings.HasPrefix(option, "!") {
					option = strings.TrimPrefix(option, "!")
					spec.SyncPolicy.SyncOptions = spec.SyncPolicy.SyncOptions.RemoveOption(option)
				} else {
					spec.SyncPolicy.SyncOptions = spec.SyncPolicy.SyncOptions.AddOption(option)
				}
			}
			if spec.SyncPolicy.IsZero() {
				spec.SyncPolicy = nil
			}
		case "sync-retry-limit":
			if appOpts.retryLimit > 0 {
				if spec.SyncPolicy == nil {
					spec.SyncPolicy = &argoappv1.SyncPolicy{}
				}
				spec.SyncPolicy.Retry = &argoappv1.RetryStrategy{
					Limit: appOpts.retryLimit,
					Backoff: &argoappv1.Backoff{
						Duration:    appOpts.retryBackoffDuration.String(),
						MaxDuration: appOpts.retryBackoffMaxDuration.String(),
						Factor:      pointer.Int64Ptr(appOpts.retryBackoffFactor),
					},
				}
			} else if appOpts.retryLimit == 0 {
				if spec.SyncPolicy.IsZero() {
					spec.SyncPolicy = nil
				} else {
					spec.SyncPolicy.Retry = nil
				}
			} else {
				log.Fatalf("Invalid sync-retry-limit [%d]", appOpts.retryLimit)
			}
		}
	})
	if flags.Changed("auto-prune") {
		if spec.SyncPolicy == nil || spec.SyncPolicy.Automated == nil {
			log.Fatal("Cannot set --auto-prune: application not configured with automatic sync")
		}
		spec.SyncPolicy.Automated.Prune = appOpts.autoPrune
	}
	if flags.Changed("self-heal") {
		if spec.SyncPolicy == nil || spec.SyncPolicy.Automated == nil {
			log.Fatal("Cannot set --self-heal: application not configured with automatic sync")
		}
		spec.SyncPolicy.Automated.SelfHeal = appOpts.selfHeal
	}
	if flags.Changed("allow-empty") {
		if spec.SyncPolicy == nil || spec.SyncPolicy.Automated == nil {
			log.Fatal("Cannot set --allow-empty: application not configured with automatic sync")
		}
		spec.SyncPolicy.Automated.AllowEmpty = appOpts.allowEmpty
	}

	return visited
}

type kustomizeOpts struct {
	namePrefix             string
	nameSuffix             string
	images                 []string
	version                string
	commonLabels           map[string]string
	commonAnnotations      map[string]string
	forceCommonLabels      bool
	forceCommonAnnotations bool
}

func setKustomizeOpt(src *argoappv1.ApplicationSource, opts kustomizeOpts) {
	if src.Kustomize == nil {
		src.Kustomize = &argoappv1.ApplicationSourceKustomize{}
	}
	if opts.version != "" {
		src.Kustomize.Version = opts.version
	}
	if opts.namePrefix != "" {
		src.Kustomize.NamePrefix = opts.namePrefix
	}
	if opts.nameSuffix != "" {
		src.Kustomize.NameSuffix = opts.nameSuffix
	}
	if opts.commonLabels != nil {
		src.Kustomize.CommonLabels = opts.commonLabels
	}
	if opts.commonAnnotations != nil {
		src.Kustomize.CommonAnnotations = opts.commonAnnotations
	}
	if opts.forceCommonLabels {
		src.Kustomize.ForceCommonLabels = opts.forceCommonLabels
	}
	if opts.forceCommonAnnotations {
		src.Kustomize.ForceCommonAnnotations = opts.forceCommonAnnotations
	}
	for _, image := range opts.images {
		src.Kustomize.MergeImage(argoappv1.KustomizeImage(image))
	}
	if src.Kustomize.IsZero() {
		src.Kustomize = nil
	}
}

func setPluginOptEnvs(src *argoappv1.ApplicationSource, envs []string) {
	if src.Plugin == nil {
		src.Plugin = &argoappv1.ApplicationSourcePlugin{}
	}

	for _, text := range envs {
		e, err := argoappv1.NewEnvEntry(text)
		if err != nil {
			log.Fatal(err)
		}
		src.Plugin.AddEnvEntry(e)
	}
}

type helmOpts struct {
	valueFiles              []string
	ignoreMissingValueFiles bool
	values                  string
	releaseName             string
	version                 string
	helmSets                []string
	helmSetStrings          []string
	helmSetFiles            []string
	passCredentials         bool
	skipCrds                bool
}

func setHelmOpt(src *argoappv1.ApplicationSource, opts helmOpts) {
	if src.Helm == nil {
		src.Helm = &argoappv1.ApplicationSourceHelm{}
	}
	if len(opts.valueFiles) > 0 {
		src.Helm.ValueFiles = opts.valueFiles
	}
	if opts.ignoreMissingValueFiles {
		src.Helm.IgnoreMissingValueFiles = opts.ignoreMissingValueFiles
	}
	if len(opts.values) > 0 {
		src.Helm.Values = opts.values
	}
	if opts.releaseName != "" {
		src.Helm.ReleaseName = opts.releaseName
	}
	if opts.version != "" {
		src.Helm.Version = opts.version
	}
	if opts.passCredentials {
		src.Helm.PassCredentials = opts.passCredentials
	}
	if opts.skipCrds {
		src.Helm.SkipCrds = opts.skipCrds
	}
	for _, text := range opts.helmSets {
		p, err := argoappv1.NewHelmParameter(text, false)
		if err != nil {
			log.Fatal(err)
		}
		src.Helm.AddParameter(*p)
	}
	for _, text := range opts.helmSetStrings {
		p, err := argoappv1.NewHelmParameter(text, true)
		if err != nil {
			log.Fatal(err)
		}
		src.Helm.AddParameter(*p)
	}
	for _, text := range opts.helmSetFiles {
		p, err := argoappv1.NewHelmFileParameter(text)
		if err != nil {
			log.Fatal(err)
		}
		src.Helm.AddFileParameter(*p)
	}
	if src.Helm.IsZero() {
		src.Helm = nil
	}
}

func setJsonnetOpt(src *argoappv1.ApplicationSource, tlaParameters []string, code bool) {
	if src.Directory == nil {
		src.Directory = &argoappv1.ApplicationSourceDirectory{}
	}
	for _, j := range tlaParameters {
		src.Directory.Jsonnet.TLAs = append(src.Directory.Jsonnet.TLAs, argoappv1.NewJsonnetVar(j, code))
	}
}

func setJsonnetOptExtVar(src *argoappv1.ApplicationSource, jsonnetExtVar []string, code bool) {
	if src.Directory == nil {
		src.Directory = &argoappv1.ApplicationSourceDirectory{}
	}
	for _, j := range jsonnetExtVar {
		src.Directory.Jsonnet.ExtVars = append(src.Directory.Jsonnet.ExtVars, argoappv1.NewJsonnetVar(j, code))
	}
}

func setJsonnetOptLibs(src *argoappv1.ApplicationSource, libs []string) {
	if src.Directory == nil {
		src.Directory = &argoappv1.ApplicationSourceDirectory{}
	}
	src.Directory.Jsonnet.Libs = append(src.Directory.Jsonnet.Libs, libs...)
}

// SetParameterOverrides updates an existing or appends a new parameter override in the application
// The app is assumed to be a helm app and is expected to be in the form:
// param=value
func SetParameterOverrides(app *argoappv1.Application, parameters []string) {
	if len(parameters) == 0 {
		return
	}
	var sourceType argoappv1.ApplicationSourceType
	if st, _ := app.Spec.Source.ExplicitType(); st != nil {
		sourceType = *st
	} else if app.Status.SourceType != "" {
		sourceType = app.Status.SourceType
	} else {
		if len(strings.SplitN(parameters[0], "=", 2)) == 2 {
			sourceType = argoappv1.ApplicationSourceTypeHelm
		}
	}

	switch sourceType {
	case argoappv1.ApplicationSourceTypeHelm:
		if app.Spec.Source.Helm == nil {
			app.Spec.Source.Helm = &argoappv1.ApplicationSourceHelm{}
		}
		for _, p := range parameters {
			newParam, err := argoappv1.NewHelmParameter(p, false)
			if err != nil {
				log.Error(err)
				continue
			}
			app.Spec.Source.Helm.AddParameter(*newParam)
		}
	default:
		log.Fatalf("Parameters can only be set against Helm applications")
	}
}

func readApps(yml []byte, apps *[]*argoappv1.Application) error {
	yamls, _ := kube.SplitYAMLToString(yml)

	var err error

	for _, yml := range yamls {
		var app argoappv1.Application
		err = config.Unmarshal([]byte(yml), &app)
		*apps = append(*apps, &app)
		if err != nil {
			return err
		}
	}

	return err
}

func readAppsFromStdin(apps *[]*argoappv1.Application) error {
	reader := bufio.NewReader(os.Stdin)
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}
	err = readApps(data, apps)
	if err != nil {
		return fmt.Errorf("unable to read manifest from stdin: %v", err)
	}
	return nil
}

func readAppsFromURI(fileURL string, apps *[]*argoappv1.Application) error {

	readFilePayload := func() ([]byte, error) {
		parsedURL, err := url.ParseRequestURI(fileURL)
		if err != nil || !(parsedURL.Scheme == "http" || parsedURL.Scheme == "https") {
			return ioutil.ReadFile(fileURL)
		}
		return config.ReadRemoteFile(fileURL)
	}

	yml, err := readFilePayload()
	if err != nil {
		return err
	}

	return readApps(yml, apps)
}

func constructAppsFromStdin() ([]*argoappv1.Application, error) {
	apps := make([]*argoappv1.Application, 0)
	// read stdin
	err := readAppsFromStdin(&apps)
	if err != nil {
		return nil, err
	}
	return apps, nil
}

func constructAppsBaseOnName(appName string, labels, annotations, args []string, appOpts AppOptions, flags *pflag.FlagSet) ([]*argoappv1.Application, error) {
	var app *argoappv1.Application
	// read arguments
	if len(args) == 1 {
		if appName != "" && appName != args[0] {
			return nil, fmt.Errorf("--name argument '%s' does not match app name %s", appName, args[0])
		}
		appName = args[0]
	}
	app = &argoappv1.Application{
		TypeMeta: v1.TypeMeta{
			Kind:       application.ApplicationKind,
			APIVersion: application.Group + "/v1alpha1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: appName,
		},
	}
	SetAppSpecOptions(flags, &app.Spec, &appOpts)
	SetParameterOverrides(app, appOpts.Parameters)
	mergeLabels(app, labels)
	setAnnotations(app, annotations)
	return []*argoappv1.Application{
		app,
	}, nil
}

func constructAppsFromFileUrl(fileURL, appName string, labels, annotations, args []string, appOpts AppOptions, flags *pflag.FlagSet) ([]*argoappv1.Application, error) {
	apps := make([]*argoappv1.Application, 0)
	// read uri
	err := readAppsFromURI(fileURL, &apps)
	if err != nil {
		return nil, err
	}
	for _, app := range apps {
		if len(args) == 1 && args[0] != app.Name {
			return nil, fmt.Errorf("app name '%s' does not match app spec metadata.name '%s'", args[0], app.Name)
		}
		if appName != "" && appName != app.Name {
			app.Name = appName
		}
		if app.Name == "" {
			return nil, fmt.Errorf("app.Name is empty. --name argument can be used to provide app.Name")
		}
		SetAppSpecOptions(flags, &app.Spec, &appOpts)
		SetParameterOverrides(app, appOpts.Parameters)
		mergeLabels(app, labels)
		setAnnotations(app, annotations)
	}
	return apps, nil
}

func ConstructApps(fileURL, appName string, labels, annotations, args []string, appOpts AppOptions, flags *pflag.FlagSet) ([]*argoappv1.Application, error) {
	if fileURL == "-" {
		return constructAppsFromStdin()
	} else if fileURL != "" {
		return constructAppsFromFileUrl(fileURL, appName, labels, annotations, args, appOpts, flags)
	}
	return constructAppsBaseOnName(appName, labels, annotations, args, appOpts, flags)
}

func mergeLabels(app *argoappv1.Application, labels []string) {
	mapLabels, err := label.Parse(labels)
	errors.CheckError(err)

	mergedLabels := make(map[string]string)

	for name, value := range app.GetLabels() {
		mergedLabels[name] = value
	}

	for name, value := range mapLabels {
		mergedLabels[name] = value
	}

	app.SetLabels(mergedLabels)
}

func setAnnotations(app *argoappv1.Application, annotations []string) {
	if len(annotations) > 0 && app.Annotations == nil {
		app.Annotations = map[string]string{}
	}
	for _, a := range annotations {
		annotation := strings.SplitN(a, "=", 2)
		if len(annotation) == 2 {
			app.Annotations[annotation[0]] = annotation[1]
		} else {
			app.Annotations[annotation[0]] = ""
		}
	}
}
