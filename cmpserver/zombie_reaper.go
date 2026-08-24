package cmpserver

import (
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// StartZombieReaper starts a background goroutine that reaps orphaned child
// processes (zombies). argocd-cmp-server runs as PID 1 in CMP sidecar
// containers; when plugin tooling forks child processes that outlive their
// parent, those processes are reparented to PID 1 and become zombies. Without
// reaping, zombies accumulate until the container's pids cgroup is exhausted
// and every fork/exec fails with EAGAIN, breaking manifest generation.
//
// See https://github.com/argoproj/argo-cd/issues/29349.
func StartZombieReaper() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGCHLD)

	go func() {
		for range sigCh {
			reapZombies()
		}
	}()
}

// reapZombies reaps all currently-exited child processes that are not actively
// awaited by os/exec. It uses Wait4 with WNOHANG so it never blocks.
func reapZombies() {
	for {
		var status syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil)
		if pid == 0 || err == syscall.ECHILD {
			return
		}
		if err != nil {
			log.WithError(err).WithField("pid", pid).Debug("error reaping child process")
			return
		}
		log.WithField("pid", pid).Debug("reaped orphaned child process")
	}
}
