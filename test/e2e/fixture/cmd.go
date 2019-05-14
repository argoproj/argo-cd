package fixture

import (
	"os/exec"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

func execCommand(workDir, name string, args ...string) (string, error) {

	start := time.Now()

	log.WithFields(log.Fields{"name": name, "args": args, "workDir": workDir}).Info("running command")

	cmd := exec.Command(name, args...)
	cmd.Dir = workDir

	outBytes, err := cmd.Output()
	output := string(outBytes)
	if err != nil {
		exErr, ok := err.(*exec.ExitError)
		if ok {
			output = output + string(exErr.Stderr)
		}
	}

	for i, line := range strings.Split(output, "\n") {
		log.Infof("%d: %s", i, line)
	}

	log.WithFields(log.Fields{"err": err, "duration": time.Since(start)}).Info("ran command")

	return output, err
}
