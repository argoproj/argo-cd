package cmpserver

import (
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestReapZombiesNoChildren(t *testing.T) {
	// reapZombies should be a no-op when there are no zombie children.
	// It should not block, not panic, and not produce any error.
	reapZombies()
}

func TestReapZombiesOrphanedChild(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("SIGCHLD/orphan-reparenting test requires Linux")
	}

	// Fork a child whose own child (grandchild) will be orphaned and
	// reparented to this process. The grandchild exits quickly, then
	// reapZombies should reap it without error.
	cmd := exec.Command("sh", "-c",
		"sh -c 'sleep 0.1 & exit 0'; exit 0")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to spawn test child: %v", err)
	}

	// Give the orphaned grandchild time to exit.
	time.Sleep(300 * time.Millisecond)

	// reapZombies should reap the grandchild without error.
	reapZombies()
}

func TestStartZombieReaper(t *testing.T) {
	// StartZombieReaper should start without error and not block.
	StartZombieReaper()
}
