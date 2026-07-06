package commands

import (
	"fmt"
	"os"
)

var verbose bool

func verboseLog(format string, args ...any) {
	if verbose {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}
