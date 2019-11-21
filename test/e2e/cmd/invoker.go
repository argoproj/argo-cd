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

func Invoke(name string, args ...string) {
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
		if !strings.HasPrefix(line, "coverage: ") && line != "PASS" {
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

func Wrap(block func() error) {
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()
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
