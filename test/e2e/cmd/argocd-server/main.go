package main

import "github.com/argoproj/argo-cd/test/e2e/cmd"

func main() {
	cmd.Invoke("./dist/argocd-server.test", "-test.coverprofile=coverage.argocd-server.out")
}
