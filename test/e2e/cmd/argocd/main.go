package main

import (
	"fmt"
	"os"

	"github.com/argoproj/argo-cd/test/e2e/cmd"
)

func main() {
	cmd.Invoke("argocd.test", fmt.Sprintf("-test.coverprofile=../../coverage.argocd.%v.out", os.Getpid()))
}
