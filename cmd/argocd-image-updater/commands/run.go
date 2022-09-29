package command

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/image-updater/argocd"
	"github.com/argoproj/argo-cd/v2/image-updater/env"
	"github.com/argoproj/argo-cd/v2/image-updater/health"
	"github.com/argoproj/argo-cd/v2/image-updater/log"
	"github.com/argoproj/argo-cd/v2/image-updater/metrics"
	"github.com/argoproj/argo-cd/v2/image-updater/registry"
	"github.com/argoproj/argo-cd/v2/image-updater/version"
	"github.com/argoproj/argo-cd/v2/reposerver/askpass"

	"github.com/spf13/cobra"

	"golang.org/x/sync/semaphore"
)

// newRunCommand implements "run" command
func newRunCommand() *cobra.Command {
	var cfg *ImageUpdaterConfig = &ImageUpdaterConfig{}
	var once bool
	var kubeConfig string
	var disableKubernetes bool
	var warmUpCache bool = true
	var commitMessagePath string
	var commitMessageTpl string
	var runCmd = &cobra.Command{
		Use:   "run",
		Short: "Runs the argocd-image-updater with a set of options",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := log.SetLogLevel(cfg.LogLevel); err != nil {
				return err
			}

			if once {
				cfg.CheckInterval = 0
				cfg.HealthPort = 0
			}

			// Enforce sane --max-concurrency values
			if cfg.MaxConcurrency < 1 {
				return fmt.Errorf("--max-concurrency must be greater than 1")
			}

			log.Infof("%s %s starting [loglevel:%s, interval:%s, healthport:%s]",
				version.BinaryName(),
				version.Version(),
				strings.ToUpper(cfg.LogLevel),
				getPrintableInterval(cfg.CheckInterval),
				getPrintableHealthPort(cfg.HealthPort),
			)

			// User can specify a path to a template used for Git commit messages
			if commitMessagePath != "" {
				tpl, err := ioutil.ReadFile(commitMessagePath)
				if err != nil {
					if errors.Is(err, os.ErrNotExist) {
						log.Warnf("commit message template at %s does not exist, using default", commitMessagePath)
						commitMessageTpl = common.DefaultGitCommitMessage
					} else {
						log.Fatalf("could not read commit message template: %v", err)
					}
				} else {
					commitMessageTpl = string(tpl)
				}
			}

			if commitMessageTpl == "" {
				log.Infof("Using default Git commit messages")
				commitMessageTpl = common.DefaultGitCommitMessage
			}

			if tpl, err := template.New("commitMessage").Parse(commitMessageTpl); err != nil {
				log.Fatalf("could not parse commit message template: %v", err)
			} else {
				log.Debugf("Successfully parsed commit message template")
				cfg.GitCommitMessage = tpl
			}

			// Load registries configuration early on. We do not consider it a fatal
			// error when the file does not exist, but we emit a warning.
			if cfg.RegistriesConf != "" {
				st, err := os.Stat(cfg.RegistriesConf)
				if err != nil || st.IsDir() {
					log.Warnf("Registry configuration at %s could not be read: %v -- using default configuration", cfg.RegistriesConf, err)
				} else {
					err = registry.LoadRegistryConfiguration(cfg.RegistriesConf, false)
					if err != nil {
						log.Errorf("Could not load registry configuration from %s: %v", cfg.RegistriesConf, err)
						return nil
					}
				}
			}

			if cfg.CheckInterval > 0 && cfg.CheckInterval < 60*time.Second {
				log.Warnf("Check interval is very low - it is not recommended to run below 1m0s")
			}

			var err error
			if !disableKubernetes {
				ctx := context.Background()
				cfg.KubeClient, err = getKubeConfig(ctx, cfg.ArgocdNamespace, kubeConfig)
				if err != nil {
					log.Fatalf("could not create K8s client: %v", err)
				}
				if cfg.ClientOpts.ServerAddr == "" {
					cfg.ClientOpts.ServerAddr = fmt.Sprintf("argocd-server.%s", cfg.KubeClient.Namespace)
				}
			}
			if cfg.ClientOpts.ServerAddr == "" {
				cfg.ClientOpts.ServerAddr = defaultArgoCDServerAddr
			}

			if token := os.Getenv("ARGOCD_TOKEN"); token != "" && cfg.ClientOpts.AuthToken == "" {
				log.Debugf("Using ArgoCD API credentials from environment ARGOCD_TOKEN")
				cfg.ClientOpts.AuthToken = token
			}

			log.Infof("ArgoCD configuration: [apiKind=%s, server=%s, auth_token=%v, insecure=%v, grpc_web=%v, plaintext=%v]",
				cfg.ApplicationsAPIKind,
				cfg.ClientOpts.ServerAddr,
				cfg.ClientOpts.AuthToken != "",
				cfg.ClientOpts.Insecure,
				cfg.ClientOpts.GRPCWeb,
				cfg.ClientOpts.Plaintext,
			)

			// Health server will start in a go routine and run asynchronously
			var hsErrCh chan error
			var msErrCh chan error
			if cfg.HealthPort > 0 {
				log.Infof("Starting health probe server TCP port=%d", cfg.HealthPort)
				hsErrCh = health.StartHealthServer(cfg.HealthPort)
			}

			if cfg.MetricsPort > 0 {
				log.Infof("Starting metrics server on TCP port=%d", cfg.MetricsPort)
				msErrCh = metrics.StartMetricsServer(cfg.MetricsPort)
			}

			if warmUpCache {
				err := warmupImageCache(cfg)
				if err != nil {
					log.Errorf("Error warming up cache: %v", err)
					return err
				}
			}

			// This is our main loop. We leave it only when our health probe server
			// returns an error.
			for {
				select {
				case err := <-hsErrCh:
					if err != nil {
						log.Errorf("Health probe server exited with error: %v", err)
					} else {
						log.Infof("Health probe server exited gracefully")
					}
					return nil
				case err := <-msErrCh:
					if err != nil {
						log.Errorf("Metrics server exited with error: %v", err)
					} else {
						log.Infof("Metrics server exited gracefully")
					}
					return nil
				default:
					if lastRun.IsZero() || time.Since(lastRun) > cfg.CheckInterval {
						result, err := runImageUpdater(cfg, false)
						if err != nil {
							log.Errorf("Error: %v", err)
						} else {
							log.Infof("Processing results: applications=%d images_considered=%d images_skipped=%d images_updated=%d errors=%d",
								result.NumApplicationsProcessed,
								result.NumImagesConsidered,
								result.NumSkipped,
								result.NumImagesUpdated,
								result.NumErrors)
						}
						lastRun = time.Now()
					}
				}
				if cfg.CheckInterval == 0 {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			log.Infof("Finished.")
			return nil
		},
	}

	runCmd.Flags().StringVar(&cfg.ApplicationsAPIKind, "applications-api", env.GetStringVal("APPLICATIONS_API", applicationsAPIKindK8S), "API kind that is used to manage Argo CD applications ('kubernetes' or 'argocd')")
	runCmd.Flags().StringVar(&cfg.ClientOpts.ServerAddr, "argocd-server-addr", env.GetStringVal("ARGOCD_SERVER", ""), "address of ArgoCD API server")
	runCmd.Flags().BoolVar(&cfg.ClientOpts.GRPCWeb, "argocd-grpc-web", env.GetBoolVal("ARGOCD_GRPC_WEB", false), "use grpc-web for connection to ArgoCD")
	runCmd.Flags().BoolVar(&cfg.ClientOpts.Insecure, "argocd-insecure", env.GetBoolVal("ARGOCD_INSECURE", false), "(INSECURE) ignore invalid TLS certs for ArgoCD server")
	runCmd.Flags().BoolVar(&cfg.ClientOpts.Plaintext, "argocd-plaintext", env.GetBoolVal("ARGOCD_PLAINTEXT", false), "(INSECURE) connect without TLS to ArgoCD server")
	runCmd.Flags().StringVar(&cfg.ClientOpts.AuthToken, "argocd-auth-token", "", "use token for authenticating to ArgoCD (unsafe - consider setting ARGOCD_TOKEN env var instead)")
	runCmd.Flags().BoolVar(&cfg.DryRun, "dry-run", false, "run in dry-run mode. If set to true, do not perform any changes")
	runCmd.Flags().DurationVar(&cfg.CheckInterval, "interval", 2*time.Minute, "interval for how often to check for updates")
	runCmd.Flags().StringVar(&cfg.LogLevel, "loglevel", env.GetStringVal("IMAGE_UPDATER_LOGLEVEL", "info"), "set the loglevel to one of trace|debug|info|warn|error")
	runCmd.Flags().StringVar(&kubeConfig, "kubeconfig", "", "full path to kubernetes client configuration, i.e. ~/.kube/config")
	runCmd.Flags().IntVar(&cfg.HealthPort, "health-port", 6400, "port to start the health server on, 0 to disable")
	runCmd.Flags().IntVar(&cfg.MetricsPort, "metrics-port", 6401, "port to start the metrics server on, 0 to disable")
	runCmd.Flags().BoolVar(&once, "once", false, "run only once, same as specifying --interval=0 and --health-port=0")
	runCmd.Flags().StringVar(&cfg.RegistriesConf, "registries-conf-path", defaultRegistriesConfPath, "path to registries configuration file")
	runCmd.Flags().BoolVar(&disableKubernetes, "disable-kubernetes", false, "do not create and use a Kubernetes client")
	runCmd.Flags().IntVar(&cfg.MaxConcurrency, "max-concurrency", 10, "maximum number of update threads to run concurrently")
	runCmd.Flags().StringVar(&cfg.ArgocdNamespace, "argocd-namespace", "", "namespace where ArgoCD runs in (current namespace by default)")
	runCmd.Flags().StringSliceVar(&cfg.AppNamePatterns, "match-application-name", nil, "patterns to match application name against")
	runCmd.Flags().StringVar(&cfg.AppLabel, "match-application-label", "", "label to match application labels against")
	runCmd.Flags().BoolVar(&warmUpCache, "warmup-cache", true, "whether to perform a cache warm-up on startup")
	runCmd.Flags().StringVar(&cfg.GitCommitUser, "git-commit-user", env.GetStringVal("GIT_COMMIT_USER", "argocd-image-updater"), "Username to use for Git commits")
	runCmd.Flags().StringVar(&cfg.GitCommitMail, "git-commit-email", env.GetStringVal("GIT_COMMIT_EMAIL", "noreply@argoproj.io"), "E-Mail address to use for Git commits")
	runCmd.Flags().StringVar(&commitMessagePath, "git-commit-message-path", defaultCommitTemplatePath, "Path to a template to use for Git commit messages")
	runCmd.Flags().BoolVar(&cfg.DisableKubeEvents, "disable-kube-events", env.GetBoolVal("IMAGE_UPDATER_KUBE_EVENTS", false), "Disable kubernetes events")

	return runCmd
}

// Main loop for argocd-image-controller
func runImageUpdater(cfg *ImageUpdaterConfig, warmUp bool) (argocd.ImageUpdaterResult, error) {
	result := argocd.ImageUpdaterResult{}
	var err error
	var argoClient argocd.ArgoCD
	switch cfg.ApplicationsAPIKind {
	case applicationsAPIKindK8S:
		argoClient, err = argocd.NewK8SClient(cfg.KubeClient)
	case applicationsAPIKindArgoCD:
		argoClient, err = argocd.NewAPIClient(&cfg.ClientOpts)
	default:
		return argocd.ImageUpdaterResult{}, fmt.Errorf("application api '%s' is not supported", cfg.ApplicationsAPIKind)
	}
	if err != nil {
		return result, err
	}
	cfg.ArgoClient = argoClient

	apps, err := cfg.ArgoClient.ListApplications()
	if err != nil {
		log.WithContext().
			AddField("argocd_server", cfg.ClientOpts.ServerAddr).
			AddField("grpc_web", cfg.ClientOpts.GRPCWeb).
			AddField("grpc_webroot", cfg.ClientOpts.GRPCWebRootPath).
			AddField("plaintext", cfg.ClientOpts.Plaintext).
			AddField("insecure", cfg.ClientOpts.Insecure).
			Errorf("error while communicating with ArgoCD")
		return result, err
	}

	// Get the list of applications that are allowed for updates, that is, those
	// applications which have correct annotation.
	appList, err := argocd.FilterApplicationsForUpdate(apps, cfg.AppNamePatterns, cfg.AppLabel)
	if err != nil {
		return result, err
	}

	metrics.Applications().SetNumberOfApplications(len(appList))

	if !warmUp {
		log.Infof("Starting image update cycle, considering %d annotated application(s) for update", len(appList))
	}

	syncState := argocd.NewSyncIterationState()

	// Allow a maximum of MaxConcurrency number of goroutines to exist at the
	// same time. If in warm-up mode, set to 1 explicitly.
	var concurrency int = cfg.MaxConcurrency
	if warmUp {
		concurrency = 1
	}
	var dryRun bool = cfg.DryRun
	if warmUp {
		dryRun = true
	}
	sem := semaphore.NewWeighted(int64(concurrency))

	var wg sync.WaitGroup
	wg.Add(len(appList))

	for app, curApplication := range appList {
		lockErr := sem.Acquire(context.TODO(), 1)
		if lockErr != nil {
			log.Errorf("Could not acquire semaphore for application %s: %v", app, lockErr)
			// Release entry in wait group on error, too - we're never gonna execute
			wg.Done()
			continue
		}

		gitCredsStore := askpass.NewServer()
		gitCredsStore.Run(askpass.SocketPath)

		go func(app string, curApplication argocd.ApplicationImages) {
			defer sem.Release(1)
			log.Debugf("Processing application %s", app)
			upconf := &argocd.UpdateConfiguration{
				NewRegFN:          registry.NewClient,
				ArgoClient:        cfg.ArgoClient,
				KubeClient:        cfg.KubeClient,
				UpdateApp:         &curApplication,
				DryRun:            dryRun,
				GitCommitUser:     cfg.GitCommitUser,
				GitCommitEmail:    cfg.GitCommitMail,
				GitCommitMessage:  cfg.GitCommitMessage,
				DisableKubeEvents: cfg.DisableKubeEvents,
				GitCredsStore:     gitCredsStore,
			}
			res := argocd.UpdateApplication(upconf, syncState)
			result.NumApplicationsProcessed += 1
			result.NumErrors += res.NumErrors
			result.NumImagesConsidered += res.NumImagesConsidered
			result.NumImagesUpdated += res.NumImagesUpdated
			result.NumSkipped += res.NumSkipped
			if !warmUp && !cfg.DryRun {
				metrics.Applications().IncreaseImageUpdate(app, res.NumImagesUpdated)
			}
			metrics.Applications().IncreaseUpdateErrors(app, res.NumErrors)
			metrics.Applications().SetNumberOfImagesWatched(app, res.NumImagesConsidered)
			wg.Done()
		}(app, curApplication)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	return result, nil
}

// warmupImageCache performs a cache warm-up, which is basically one cycle of
// the image update process with dryRun set to true and a maximum concurrency
// of 1, i.e. sequential processing.
func warmupImageCache(cfg *ImageUpdaterConfig) error {
	log.Infof("Warming up image cache")
	_, err := runImageUpdater(cfg, true)
	if err != nil {
		return nil
	}
	entries := 0
	eps := registry.ConfiguredEndpoints()
	for _, ep := range eps {
		r, err := registry.GetRegistryEndpoint(ep)
		if err == nil {
			entries += r.Cache.NumEntries()
		}
	}
	log.Infof("Finished cache warm-up, pre-loaded %d meta data entries from %d registries", entries, len(eps))
	return nil
}
