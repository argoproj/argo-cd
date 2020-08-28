package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"
	"github.com/argoproj/gitops-engine/pkg/sync"
	"github.com/argoproj/gitops-engine/pkg/utils/errors"
	"github.com/argoproj/gitops-engine/pkg/utils/io"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
)

const (
	annotationGCMark = "gitops-agent.argoproj.io/gc-mark"
)

func main() {
	if err := newCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
			if ext := filepath.Ext(info.Name()); ext != ".json" && ext != ".yml" && ext != ".yaml" {
				return nil
			}
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			items, err := kube.SplitYAML(data)
			if err != nil {
				return fmt.Errorf("failed to parse %s: %v", path, err)
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

func newCmd() *cobra.Command {
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
			errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
			if namespace == "" {
				namespace, _, err = clientConfig.Namespace()
				errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
			}

			var namespaces []string
			if namespaced {
				namespaces = []string{namespace}
			}
			clusterCache := cache.NewClusterCache(config,
				cache.SetNamespaces(namespaces),
				cache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, isRoot bool) (info interface{}, cacheManifest bool) {
					// store gc mark of every resource
					gcMark := un.GetAnnotations()[annotationGCMark]
					info = &resourceInfo{gcMark: un.GetAnnotations()[annotationGCMark]}
					// cache resources that has that mark to improve performance
					cacheManifest = gcMark != ""
					return
				}),
			)
			gitOpsEngine := engine.NewEngine(config, clusterCache)
			errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)

			closer, err := gitOpsEngine.Run()
			errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)

			defer io.Close(closer)

			resync := make(chan bool)
			go func() {
				ticker := time.NewTicker(time.Second * time.Duration(resyncSeconds))
				for {
					<-ticker.C
					log.Infof("Synchronization triggered by timer")
					resync <- true
				}
			}()
			http.HandleFunc("/api/v1/sync", func(writer http.ResponseWriter, request *http.Request) {
				log.Infof("Synchronization triggered by API call")
				resync <- true
			})
			go func() {
				errors.CheckErrorWithCode(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), nil), errors.ErrorCommandSpecific)
			}()

			for ; true; <-resync {
				target, revision, err := s.parseManifests()
				if err != nil {
					log.Warnf("failed to parse target state: %v", err)
					continue
				}

				result, err := gitOpsEngine.Sync(context.Background(), target, func(r *cache.Resource) bool {
					return r.Info.(*resourceInfo).gcMark == s.getGCMark(r.ResourceKey())
				}, revision, namespace, sync.WithPrune(prune))
				if err != nil {
					log.Warnf("failed to synchronize cluster state: %v", err)
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
