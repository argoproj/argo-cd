package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/argoproj/gitops-engine/pkg/utils/text"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2/textlogger"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"
	"github.com/argoproj/gitops-engine/pkg/sync"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"

	_ "net/http/pprof"
)

const (
	annotationGCMark = "gitops-agent.argoproj.io/gc-mark"
	envProfile       = "GITOPS_ENGINE_PROFILE"
	envProfileHost   = "GITOPS_ENGINE_PROFILE_HOST"
	envProfilePort   = "GITOPS_ENGINE_PROFILE_PORT"
)

func main() {
	log := textlogger.NewLogger(textlogger.NewConfig())
	err := newCmd(log).Execute()
	checkError(err, log)
}

type resourceInfo struct {
	gcMark string
}

type settings struct {
	repoPath string
	paths    []string
}

func (s *settings) getGCMark(key kube.ResourceKey) string {
	h := sha256.New()
	_, _ = h.Write([]byte(fmt.Sprintf("%s/%s", s.repoPath, strings.Join(s.paths, ","))))
	_, _ = h.Write([]byte(strings.Join([]string{key.Group, key.Kind, key.Name}, "/")))
	return "sha256." + base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func (s *settings) parseManifests() ([]*unstructured.Unstructured, string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = s.repoPath
	revision, err := cmd.CombinedOutput()
	if err != nil {
		return nil, "", err
	}
	var res []*unstructured.Unstructured
	for i := range s.paths {
		if err := filepath.Walk(filepath.Join(s.repoPath, s.paths[i]), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if ext := strings.ToLower(filepath.Ext(info.Name())); ext != ".json" && ext != ".yml" && ext != ".yaml" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			items, err := kube.SplitYAML(data)
			if err != nil {
				return fmt.Errorf("failed to parse %s: %w", path, err)
			}
			res = append(res, items...)
			return nil
		}); err != nil {
			return nil, "", err
		}
	}
	for i := range res {
		annotations := res[i].GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[annotationGCMark] = s.getGCMark(kube.GetResourceKey(res[i]))
		res[i].SetAnnotations(annotations)
	}
	return res, string(revision), nil
}

func StartProfiler(log logr.Logger) {
	if os.Getenv(envProfile) == "web" {
		go func() {
			runtime.SetBlockProfileRate(1)
			runtime.SetMutexProfileFraction(1)
			profilePort := text.WithDefault(os.Getenv(envProfilePort), "6060")
			profileHost := text.WithDefault(os.Getenv(envProfileHost), "127.0.0.1")

			log.Info("pprof", "err", http.ListenAndServe(fmt.Sprintf("%s:%s", profileHost, profilePort), nil))
		}()
	}
}

func newCmd(log logr.Logger) *cobra.Command {
	var (
		clientConfig  clientcmd.ClientConfig
		paths         []string
		resyncSeconds int
		port          int
		prune         bool
		namespace     string
		namespaced    bool
	)
	cmd := cobra.Command{
		Use: "gitops REPO_PATH",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				cmd.HelpFunc()(cmd, args)
				os.Exit(1)
			}
			s := settings{args[0], paths}
			config, err := clientConfig.ClientConfig()
			checkError(err, log)
			if namespace == "" {
				namespace, _, err = clientConfig.Namespace()
				checkError(err, log)
			}

			var namespaces []string
			if namespaced {
				namespaces = []string{namespace}
			}

			StartProfiler(log)
			clusterCache := cache.NewClusterCache(config,
				cache.SetNamespaces(namespaces),
				cache.SetLogr(log),
				cache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {
					// store gc mark of every resource
					gcMark := un.GetAnnotations()[annotationGCMark]
					info = &resourceInfo{gcMark: un.GetAnnotations()[annotationGCMark]}
					// cache resources that has that mark to improve performance
					cacheManifest = gcMark != ""
					return
				}),
			)
			gitOpsEngine := engine.NewEngine(config, clusterCache, engine.WithLogr(log))
			checkError(err, log)

			cleanup, err := gitOpsEngine.Run()
			checkError(err, log)
			defer cleanup()

			resync := make(chan bool)
			go func() {
				ticker := time.NewTicker(time.Second * time.Duration(resyncSeconds))
				for {
					<-ticker.C
					log.Info("Synchronization triggered by timer")
					resync <- true
				}
			}()
			http.HandleFunc("/api/v1/sync", func(_ http.ResponseWriter, _ *http.Request) {
				log.Info("Synchronization triggered by API call")
				resync <- true
			})
			go func() {
				checkError(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), nil), log)
			}()

			for ; true; <-resync {
				target, revision, err := s.parseManifests()
				if err != nil {
					log.Error(err, "Failed to parse target state")
					continue
				}

				result, err := gitOpsEngine.Sync(context.Background(), target, func(r *cache.Resource) bool {
					return r.Info.(*resourceInfo).gcMark == s.getGCMark(r.ResourceKey())
				}, revision, namespace, sync.WithPrune(prune), sync.WithLogr(log))
				if err != nil {
					log.Error(err, "Failed to synchronize cluster state")
					continue
				}
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				_, _ = fmt.Fprintf(w, "RESOURCE\tRESULT\n")
				for _, res := range result {
					_, _ = fmt.Fprintf(w, "%s\t%s\n", res.ResourceKey.String(), res.Message)
				}
				_ = w.Flush()
			}
		},
	}
	clientConfig = addKubectlFlagsToCmd(&cmd)
	cmd.Flags().StringArrayVar(&paths, "path", []string{"."}, "Directory path with-in repository")
	cmd.Flags().IntVar(&resyncSeconds, "resync-seconds", 300, "Resync duration in seconds.")
	cmd.Flags().IntVar(&port, "port", 9001, "Port number.")
	cmd.Flags().BoolVar(&prune, "prune", true, "Enables resource pruning.")
	cmd.Flags().BoolVar(&namespaced, "namespaced", false, "Switches agent into namespaced mode.")
	cmd.Flags().StringVar(&namespace, "default-namespace", "",
		"The namespace that should be used if resource namespace is not specified. "+
			"By default resources are installed into the same namespace where gitops-agent is installed.")
	return &cmd
}

// addKubectlFlagsToCmd adds kubectl like flags to a command and returns the ClientConfig interface
// for retrieving the values.
func addKubectlFlagsToCmd(cmd *cobra.Command) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := clientcmd.ConfigOverrides{}
	kflags := clientcmd.RecommendedConfigOverrideFlags("")
	cmd.PersistentFlags().StringVar(&loadingRules.ExplicitPath, "kubeconfig", "", "Path to a kube config. Only required if out-of-cluster")
	clientcmd.BindOverrideFlags(&overrides, cmd.PersistentFlags(), kflags)
	return clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)
}

// checkError is a convenience function to check if an error is non-nil and exit if it was
func checkError(err error, log logr.Logger) {
	if err != nil {
		log.Error(err, "Fatal error")
		os.Exit(1)
	}
}
