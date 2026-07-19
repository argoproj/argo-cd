package main

import (
	"fmt"
	"os"

	"github.com/argoproj/argo-cd/v3/util/configbus"
)

func main() {
	if err := configbus.WriteReferenceDoc(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
