package main

import (
	"fmt"
	"time"

	"github.com/argoproj/argo-cd/test/e2e/cmd"
)

func main() {
	cmd.DelegateToTest("argocd.test", fmt.Sprintf("-test.coverprofile=../../coverage.argocd.%v.out", time.Now().Second()))
}
