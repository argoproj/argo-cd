package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-cd/v2/util/errors"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func main() {
	testEnv := envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("manifests", "crds")},
	}
	println("Starting K8S...")
	cfg, err := testEnv.Start()

	errors.CheckError(err)
	kubeConfigPath := "/tmp/kubeconfig"
	if len(os.Args) > 2 {
		kubeConfigPath = os.Args[1]
	}

	println(fmt.Sprintf("Kubeconfig is available at %s", kubeConfigPath))
	errors.CheckError(kube.WriteKubeConfig(cfg, "default", kubeConfigPath))
	client, err := kubernetes.NewForConfig(cfg)
	errors.CheckError(err)

	attempts := 5
	interval := time.Second
	for i := 0; i < attempts; i++ {
		_, err = client.ServerVersion()
		if err == nil {
			break
		}
		time.Sleep(interval)
	}
	errors.CheckError(err)

	cmd := exec.Command("kubectl", "apply", "-k", "manifests/base/config")
	cmd.Env = []string{fmt.Sprintf("KUBECONFIG=%s", kubeConfigPath)}
	errors.CheckError(cmd.Run())
	<-context.Background().Done()
}
