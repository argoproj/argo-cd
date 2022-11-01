package admin

import (
	"context"
	"fmt"
	"io/ioutil"
	"runtime"
	"text/template"
	"time"

	command "github.com/argoproj/argo-cd/v2/cmd/argocd-image-updater/commands"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/image-updater/argocd"
	"github.com/argoproj/argo-cd/v2/image-updater/image"
	"github.com/argoproj/argo-cd/v2/image-updater/kube"
	"github.com/argoproj/argo-cd/v2/image-updater/log"
	"github.com/argoproj/argo-cd/v2/image-updater/options"
	"github.com/argoproj/argo-cd/v2/image-updater/registry"
	"github.com/argoproj/argo-cd/v2/image-updater/tag"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/ratelimit"
)

func NewImageUpdaterCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "image-updater",
		Short: "Automatically update container images with ArgoCD",
	}

	command.AddCommand(newTestCommand())
	command.AddCommand(newTemplateCommand())
	return command
}

func newTestCommand() *cobra.Command {
	var (
		semverConstraint   string
		strategy           string
		registriesConfPath string
		logLevel           string
		allowTags          string
		credentials        string
		kubeConfig         string
		disableKubernetes  bool
		ignoreTags         []string
		disableKubeEvents  bool
		rateLimit          int
		platforms          []string
	)
	var runCmd = &cobra.Command{
		Use:   "test IMAGE",
		Short: "Test the behaviour of argocd-image-updater",
		Long: `
The test command lets you test the behaviour of argocd-image-updater before
configuring annotations on your Argo CD Applications.

Its main use case is to tell you to which tag a given image would be updated
to using the given parametrization. Command line switches can be used as a
way to supply the required parameters.
`,
		Example: `
# In the most simple form, check for the latest available (semver) version of
# an image in the registry
argocd-image-updater test nginx

# Check to which version the nginx image within the 1.17 branch would be
# updated to, using the default semver strategy
argocd-image-updater test nginx --semver-constraint v1.17.x

# Check for the latest built image for a tag that matches a pattern
argocd-image-updater test nginx --allow-tags '^1.19.\d+(\-.*)*$' --update-strategy latest
`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 1 {
				cmd.HelpFunc()(cmd, args)
				log.Fatalf("image needs to be specified")
			}

			if err := log.SetLogLevel(logLevel); err != nil {
				log.Fatalf("could not set log level to %s: %v", logLevel, err)
			}

			var kubeClient *kube.KubernetesClient
			var err error
			if !disableKubernetes {
				ctx := context.Background()
				kubeClient, err = command.GetKubeConfig(ctx, "", kubeConfig)
				if err != nil {
					log.Fatalf("could not create K8s client: %v", err)
				}
			}

			img := image.NewFromIdentifier(args[0])

			vc := &image.VersionConstraint{
				Constraint: semverConstraint,
				Strategy:   image.StrategySemVer,
			}

			vc.Strategy = img.ParseUpdateStrategy(strategy)

			if allowTags != "" {
				vc.MatchFunc, vc.MatchArgs = img.ParseMatchfunc(allowTags)
			}

			vc.IgnoreList = ignoreTags

			logCtx := img.LogContext()
			logCtx.Infof("retrieving information about image")

			vc.Options = options.NewManifestOptions()
			for _, platform := range platforms {
				os, arch, variant, err := image.ParsePlatform(platform)
				if err != nil {
					logCtx.Fatalf("Could not parse platform %s: %v", platform, err)
				}
				if os != "linux" && os != "windows" {
					log.Warnf("Target platform is '%s/%s', but that's not a supported container platform. Forgot --platforms?", os, arch)
				}
				vc.Options = vc.Options.WithPlatform(os, arch, variant)
			}
			vc.Options = vc.Options.WithMetadata(vc.Strategy.NeedsMetadata())

			vc.Options.WithLogger(logCtx.AddField("application", "test"))

			if registriesConfPath != "" {
				if err := registry.LoadRegistryConfiguration(registriesConfPath, false); err != nil {
					logCtx.Fatalf("could not load registries configuration: %v", err)
				}
			}

			ep, err := registry.GetRegistryEndpoint(img.RegistryURL)
			if err != nil {
				logCtx.Fatalf("could not get registry endpoint: %v", err)
			}

			if err := ep.SetEndpointCredentials(kubeClient); err != nil {
				logCtx.Fatalf("could not set registry credentials: %v", err)
			}

			checkFlag := func(f *pflag.Flag) {
				if f.Name == "rate-limit" {
					logCtx.Infof("Overriding registry rate-limit to %d requests per second", rateLimit)
					ep.Limiter = ratelimit.New(rateLimit)
				}
			}

			cmd.Flags().Visit(checkFlag)

			var creds *image.Credential
			var username, password string
			if credentials != "" {
				credSrc, err := image.ParseCredentialSource(credentials, false)
				if err != nil {
					logCtx.Fatalf("could not parse credential definition '%s': %v", credentials, err)
				}
				creds, err = credSrc.FetchCredentials(ep.RegistryAPI, kubeClient)
				if err != nil {
					logCtx.Fatalf("could not fetch credentials: %v", err)
				}
				username = creds.Username
				password = creds.Password
			}

			regClient, err := registry.NewClient(ep, username, password)
			if err != nil {
				logCtx.Fatalf("could not create registry client: %v", err)
			}

			logCtx.Infof("Fetching available tags and metadata from registry")

			tags, err := ep.GetTags(img, regClient, vc)
			if err != nil {
				logCtx.Fatalf("could not get tags: %v", err)
			}

			logCtx.Infof("Found %d tags in registry", len(tags.Tags()))

			upImg, err := img.GetNewestVersionFromTags(vc, tags)
			if err != nil {
				logCtx.Fatalf("could not get updateable image from tags: %v", err)
			}
			if upImg == nil {
				logCtx.Infof("no newer version of image found")
				return
			}

			logCtx.Infof("latest image according to constraint is %s", img.WithTag(upImg))
		},
	}

	runCmd.Flags().StringVar(&semverConstraint, "semver-constraint", "", "only consider tags matching semantic version constraint")
	runCmd.Flags().StringVar(&allowTags, "allow-tags", "", "only consider tags in registry that satisfy the match function")
	runCmd.Flags().StringArrayVar(&ignoreTags, "ignore-tags", nil, "ignore tags in registry that match given glob pattern")
	runCmd.Flags().StringVar(&strategy, "update-strategy", "semver", "update strategy to use, one of: semver, latest)")
	runCmd.Flags().StringVar(&registriesConfPath, "registries-conf-path", "", "path to registries configuration")
	runCmd.Flags().StringVar(&logLevel, "loglevel", "debug", "log level to use (one of trace, debug, info, warn, error)")
	runCmd.Flags().BoolVar(&disableKubernetes, "disable-kubernetes", false, "whether to disable the Kubernetes client")
	runCmd.Flags().StringVar(&kubeConfig, "kubeconfig", "", "path to your Kubernetes client configuration")
	runCmd.Flags().StringVar(&credentials, "credentials", "", "the credentials definition for the test (overrides registry config)")
	runCmd.Flags().StringSliceVar(&platforms, "platforms", []string{fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)}, "limit images to given platforms")
	runCmd.Flags().BoolVar(&disableKubeEvents, "disable-kubernetes-events", false, "Disable kubernetes events")
	runCmd.Flags().IntVar(&rateLimit, "rate-limit", 20, "specificy registry rate limit (overrides registry.conf)")
	return runCmd
}

func newTemplateCommand() *cobra.Command {
	var (
		commitMessageTemplatePath string
		tplStr                    string
	)
	var runCmd = &cobra.Command{
		Use:   "template [<PATH>]",
		Short: "Test & render a commit message template",
		Long: `
The template command lets you validate your commit message template. It will
parse the template at given PATH and execute it with a defined set of changes
so that you can see how it looks like when being templated by Image Updater.

If PATH is not given, will show you the default message that is used.
`,
		Run: func(cmd *cobra.Command, args []string) {
			var tpl *template.Template
			var err error
			if len(args) != 1 {
				tplStr = common.DefaultGitCommitMessage
			} else {
				commitMessageTemplatePath = args[0]
				tplData, err := ioutil.ReadFile(commitMessageTemplatePath)
				if err != nil {
					log.Fatalf("%v", err)
				}
				tplStr = string(tplData)
			}
			if tpl, err = template.New("commitMessage").Parse(tplStr); err != nil {
				log.Fatalf("could not parse commit message template: %v", err)
			}
			chL := []argocd.ChangeEntry{
				{
					Image:  image.NewFromIdentifier("gcr.io/example/example:1.0.0"),
					OldTag: tag.NewImageTag("1.0.0", time.Now(), ""),
					NewTag: tag.NewImageTag("1.0.1", time.Now(), ""),
				},
				{
					Image:  image.NewFromIdentifier("gcr.io/example/updater@sha256:f2ca1bb6c7e907d06dafe4687e579fce76b37e4e93b7605022da52e6ccc26fd2"),
					OldTag: tag.NewImageTag("", time.Now(), "sha256:01d09d19c2139a46aebfb577780d123d7396e97201bc7ead210a2ebff8239dee"),
					NewTag: tag.NewImageTag("", time.Now(), "sha256:7aa7a5359173d05b63cfd682e3c38487f3cb4f7f1d60659fe59fab1505977d4c"),
				},
			}
			fmt.Printf("%s\n", argocd.TemplateCommitMessage(tpl, "example-app", chL))
		},
	}
	return runCmd
}
