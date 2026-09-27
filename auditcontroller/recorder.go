// Package auditcontroller implements the argocd-audit-controller: the single
// place where Argo CD's audit trail ("who did what") is written out.
//
// Records arrive from two sources:
//
//   - the API server, which ships a record for every mutating API call and web
//     terminal session (see server/audit), over an authenticated HTTP endpoint;
//   - Kubernetes Events emitted by the Argo CD controllers (automated syncs,
//     sync results, ...), which the controller watches.
//
// Every record is written as one JSON line to stdout (so the audit trail is in
// the pod's logs, `kubectl logs deploy/argocd-audit-controller`), optionally
// appended to a file, kept in a bounded in-memory buffer that can be queried
// over HTTP, and counted in Prometheus metrics.
package auditcontroller

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/argoproj/argo-cd/v3/util/audit"
)

// Recorder writes audit records to its outputs and remembers the most recent ones.
type Recorder struct {
	mu      sync.Mutex
	outputs []io.Writer
	ring    []audit.Record
	next    int
	full    bool
	metrics *Metrics
	now     func() time.Time
}

// NewRecorder returns a Recorder writing JSON lines to outputs and keeping the
// last bufferSize records in memory.
func NewRecorder(bufferSize int, metrics *Metrics, outputs ...io.Writer) *Recorder {
	if bufferSize < 1 {
		bufferSize = 1
	}
	return &Recorder{
		outputs: outputs,
		ring:    make([]audit.Record, bufferSize),
		metrics: metrics,
		now:     time.Now,
	}
}

// Record normalizes and writes a record. It returns an error if the record
// could not be written to every output.
func (r *Recorder) Record(rec audit.Record) error {
	now := r.now().UTC()
	rec.Kind = audit.RecordKind
	if rec.ID == "" {
		rec.ID = uuid.NewString()
	}
	if rec.Timestamp.IsZero() {
		rec.Timestamp = now
	}
	rec.ReceivedAt = &now
	if rec.Result.Status == "" {
		rec.Result.Status = audit.ResultSuccess
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("failed to marshal audit record: %w", err)
	}
	line = append(line, '\n')

	r.mu.Lock()
	defer r.mu.Unlock()
	var writeErr error
	for _, out := range r.outputs {
		if _, err := out.Write(line); err != nil && writeErr == nil {
			writeErr = fmt.Errorf("failed to write audit record: %w", err)
		}
	}
	r.ring[r.next] = rec
	r.next = (r.next + 1) % len(r.ring)
	if r.next == 0 {
		r.full = true
	}
	if r.metrics != nil {
		r.metrics.recordWritten(rec, writeErr)
	}
	return writeErr
}

// Filter selects records returned by Recent. Empty fields match everything.
type Filter struct {
	User         string
	Action       string
	Verb         string
	ResourceType string
	ResourceName string
	Project      string
	Result       string
	Since        time.Time
}

func (f Filter) matches(rec audit.Record) bool {
	eq := func(want, got string) bool { return want == "" || strings.EqualFold(want, got) }
	return eq(f.User, rec.Actor.Username) &&
		eq(f.Action, rec.Action) &&
		eq(f.Verb, rec.Verb) &&
		eq(f.ResourceType, rec.Resource.Type) &&
		eq(f.ResourceName, rec.Resource.Name) &&
		eq(f.Project, rec.Resource.Project) &&
		eq(f.Result, rec.Result.Status) &&
		(f.Since.IsZero() || !rec.Timestamp.Before(f.Since))
}

// Recent returns up to limit buffered records matching filter, newest first.
func (r *Recorder) Recent(filter Filter, limit int) []audit.Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	size := r.next
	if r.full {
		size = len(r.ring)
	}
	result := []audit.Record{}
	for i := 0; i < size && (limit <= 0 || len(result) < limit); i++ {
		idx := (r.next - 1 - i + len(r.ring)) % len(r.ring)
		if filter.matches(r.ring[idx]) {
			result = append(result, r.ring[idx])
		}
	}
	return result
}
