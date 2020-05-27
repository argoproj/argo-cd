package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/argoproj/pkg/kube/cli"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/gitops-engine/pkg/engine"
	"github.com/argoproj/gitops-engine/pkg/utils/errors"
	executil "github.com/argoproj/gitops-engine/pkg/utils/exec"
	"github.com/argoproj/gitops-engine/pkg/utils/io"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/kube/cache"
)

func nothingManaged(_ *cache.Resource) bool {
	return false
}

func main() {
	if err := newCmd().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func newCmd() *cobra.Command {
	var (
		clientConfig  clientcmd.ClientConfig
		paths         []string
		resyncSeconds int
		port          int
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
			repoPath := args[0]
			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			if namespace == "" {
				namespace, _, err = clientConfig.Namespace()
				errors.CheckError(err)
			}

			var namespaces []string
			if namespaced {
				namespaces = []string{namespace}
			}
			gitOpsEngine := engine.NewEngine(config, cache.NewClusterCache(config, cache.SetNamespaces(namespaces)))
			errors.CheckError(err)

			closer, err := gitOpsEngine.Run()
			errors.CheckError(err)

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
				errors.CheckError(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), nil))
			}()

			for ; true; <-resync {
				target, revision, err := parseManifests(repoPath, paths)
				if err != nil {
					log.Warnf("failed to parse target state: %v", err)
					continue
				}

				result, err := gitOpsEngine.Sync(context.Background(), target, nothingManaged, revision, namespace)
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
	clientConfig = cli.AddKubectlFlagsToCmd(&cmd)
	cmd.Flags().StringArrayVar(&paths, "path", []string{"."}, "Directory path with-in repository")
	cmd.Flags().IntVar(&resyncSeconds, "resync-seconds", 300, "Resync duration in seconds.")
	cmd.Flags().IntVar(&port, "port", 9001, "Port number.")
	cmd.Flags().BoolVar(&namespaced, "namespaced", false, "Switches agent into namespaced mode.")
	cmd.Flags().StringVar(&namespace, "default-namespace", "",
		"The namespace that should be used if resource namespace is not specified. "+
			"By default resources are installed into the same namespace where gitops-agent is installed.")
	return &cmd
}

func parseManifests(repoPath string, paths []string) ([]*unstructured.Unstructured, string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	revision, err := executil.Run(cmd)
	if err != nil {
		return nil, "", err
	}
	var res []*unstructured.Unstructured
	for i := range paths {
		if err := filepath.Walk(filepath.Join(repoPath, paths[i]), func(path string, info os.FileInfo, err error) error {
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
			items, err := kube.SplitYAML(string(data))
			if err != nil {
				return fmt.Errorf("failed to parse %s: %v", path, err)
			}
			res = append(res, items...)
			return nil
		}); err != nil {
			return nil, "", err
		}
	}
	return res, revision, nil
}
