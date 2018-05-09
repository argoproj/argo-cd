package main

import (
	"bufio"
	"fmt"
	"hash/fnv"
	"os"
	"strings"

	"github.com/argoproj/argo-cd/util/git"
)

// origRepoURLToSecretName hashes repo URL to the secret name using a formula.
// Part of the original repo name is incorporated for debugging purposes
func origRepoURLToSecretName(repo string) string {
	repo = git.NormalizeGitURL(repo)
	h := fnv.New32a()
	_, _ = h.Write([]byte(repo))
	parts := strings.Split(strings.TrimSuffix(repo, ".git"), "/")
	return fmt.Sprintf("repo-%s-%v", strings.ToLower(parts[len(parts)-1]), h.Sum32())
}

// repoURLToSecretName hashes repo URL to the secret name using a formula.
// Part of the original repo name is incorporated for debugging purposes
func repoURLToSecretName(repo string) string {
	repo = strings.ToLower(git.NormalizeGitURL(repo))
	h := fnv.New32a()
	_, _ = h.Write([]byte(repo))
	parts := strings.Split(strings.TrimSuffix(repo, ".git"), "/")
	return fmt.Sprintf("repo-%s-%v", parts[len(parts)-1], h.Sum32())
}

// InputString requests an input from the user
// For security reasons, please do not use for password input
func inputString(prompt string, printArgs ...interface{}) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf(prompt, printArgs...)
	inputRaw, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(inputRaw)
}
