package reaper

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// reapInterval is a safety net: SIGCHLD is not queued, so signals for
// children that exit while a reap pass is already running can coalesce. A
// periodic pass guarantees every orphaned zombie is eventually collected.
const reapInterval = 30 * time.Second

// StartIfPID1 starts the zombie reaper if the current process is PID 1 and
// reports whether it did so. It must be called before any children are
// spawned so no SIGCHLD is missed. The reaper stops when ctx is done.
func StartIfPID1(ctx context.Context) bool {
	if os.Getpid() != 1 {
		return false
	}
	start(ctx)
	return true
}

func start(ctx context.Context) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, unix.SIGCHLD)
	go run(ctx, sigCh)
}

func run(ctx context.Context, sigCh chan os.Signal) {
	ticker := time.NewTicker(reapInterval)
	defer ticker.Stop()
	defer signal.Stop(sigCh)
	for {
		select {
		case <-ctx.Done():
			return
		case <-sigCh:
		case <-ticker.C:
		}
		reapOrphans()
	}
}

// reapOrphans collects every zombie child of the current process that is not
// registered as directly managed, and returns how many it reaped. Waiting is
// done per-pid (never wait4(-1)) so exit statuses of tracked children are
// left for their os/exec owners.
func reapOrphans() int {
	// Block spawners so no child can exist without being tracked yet.
	spawnMu.Lock()
	defer spawnMu.Unlock()
	reaped := 0
	for _, pid := range zombieChildren(os.Getpid()) {
		if isTracked(pid) {
			continue
		}
		var status unix.WaitStatus
		wpid, err := unix.Wait4(pid, &status, unix.WNOHANG, nil)
		// ECHILD/ESRCH mean the process is already gone; WNOHANG returning 0
		// means it is not waitable yet. All are fine to skip.
		if err == nil && wpid == pid {
			reaped++
			log.WithFields(log.Fields{"pid": pid, "exitCode": status.ExitStatus()}).Info("Reaped orphaned zombie child process")
		}
	}
	return reaped
}

// zombieChildren scans /proc for processes in state Z whose parent is ppid.
func zombieChildren(ppid int) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		log.Warnf("Zombie reaper failed to read /proc: %v", err)
		return nil
	}
	var pids []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		data, err := os.ReadFile("/proc/" + entry.Name() + "/stat")
		if err != nil {
			// The process disappeared between the scan and the read.
			continue
		}
		state, parent, ok := parseProcStat(string(data))
		if ok && state == 'Z' && parent == ppid {
			pids = append(pids, pid)
		}
	}
	return pids
}

// parseProcStat extracts the state and ppid fields from the content of a
// /proc/<pid>/stat file. The comm field (2nd) may contain spaces and
// parentheses, so fields are parsed from the last ')'.
func parseProcStat(stat string) (state byte, ppid int, ok bool) {
	i := strings.LastIndexByte(stat, ')')
	if i < 0 || i+1 >= len(stat) {
		return 0, 0, false
	}
	fields := strings.Fields(stat[i+1:])
	if len(fields) < 2 {
		return 0, 0, false
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, false
	}
	return fields[0][0], ppid, true
}
