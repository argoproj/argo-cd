package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

const key = "ARGS"
const delimiter = "★"

/*
	To get coverage of CLI code, we must do several things:

	(1)

	We can only get coverage using `go test` so we must create a binary, but one that does nothing other than run
	the main package. We don't want to run this test by default, so we use the `e2efixtures` build flag to prevent this.

	By convention, we create a binary named `{binaryName}.test`.

	See `RunCommand`.

	(2)

	We need a binary to replace the original binary. This invokes the test binary, piping stdin/stdout/stderr.

	See `DelegateToTest`
*/

func RunCommand(block func() error) {
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	// Coverage data is only saved if the CLI cleanly exits, so we need code to trap SIGEXIT and SIGINT and cleanly exit.
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()
	// Because test binaries take different arguments, read the args from an environment variable
	// and puts them into back into `os.Args`.
	args := os.Getenv(key)
	if len(args) > 0 {
		os.Args = append(os.Args[0:1], strings.Split(args, delimiter)...)
	} else {
		os.Args = os.Args[0:1]
	}
	go func() {
		err := block()
		if err != nil {
			panic(err)
		}
		done <- true
	}()
	<-done
}

func DelegateToTest(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), key+"="+strings.Join(os.Args[1:], delimiter))
	cmd.Stdin = os.Stdin
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	in := bufio.NewScanner(stdout)
	for in.Scan() {
		line := in.Text()
		// As test binaries print "PASS", "FAIL" and "coverage" to stdout, we remove these.
		if !strings.HasPrefix(line, "coverage: ") && !strings.HasPrefix(line, "FAIL") && line != "PASS" {
			fmt.Println(line)
		}
	}
	err = in.Err()
	if err != nil {
		panic(err)
	}
	err = cmd.Wait()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		} else {
			panic(err)
		}
	}
	os.Exit(0)
}
