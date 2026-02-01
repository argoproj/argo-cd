package kubetest

import (
	"context"
	"sync"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// MockSeqResourceOpts is a small test-only implementation of kube.ResourceOperations
// that extends the existing MockResourceOps by allowing the configuration of a sequence of errors that will be returned
// from CreateResource on successive calls. It records the last command per resource key similar to the
// existing mock, so assertions can be made.
type MockSeqResourceOpts struct {
	MockResourceOps

	createSeq   []error
	createIdx   int
	mu          sync.Mutex
	lastCommand map[kube.ResourceKey]string
}

func NewSeqTestResourceOps() *MockSeqResourceOpts {
	return &MockSeqResourceOpts{
		createSeq:   nil,
		createIdx:   0,
		lastCommand: map[kube.ResourceKey]string{},
	}
}

func (s *MockSeqResourceOpts) SetCreateBehaviorSequence(seq []error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if seq == nil {
		s.createSeq = nil
		s.createIdx = 0
		return
	}
	s.createSeq = make([]error, len(seq))
	copy(s.createSeq, seq)
	s.createIdx = 0
}

func (s *MockSeqResourceOpts) nextCreateBehavior() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.createSeq == nil || s.createIdx >= len(s.createSeq) {
		return nil
	}
	err := s.createSeq[s.createIdx]
	s.createIdx++
	return err
}

func (s *MockSeqResourceOpts) SetLastResourceCommand(key kube.ResourceKey, cmd string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastCommand == nil {
		s.lastCommand = map[kube.ResourceKey]string{}
	}
	s.lastCommand[key] = cmd
}

func (s *MockSeqResourceOpts) GetLastResourceCommand(key kube.ResourceKey) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastCommand[key]
}

// CreateResource consults sequence first, then behave accordingly.
func (s *MockSeqResourceOpts) CreateResource(_ context.Context, obj *unstructured.Unstructured, dryRun cmdutil.DryRunStrategy, validate bool) (string, error) {
	if dryRun != cmdutil.DryRunNone && !s.ExecuteForDryRun {
		return "", nil
	}

	if err := s.nextCreateBehavior(); err != nil {
		s.SetLastResourceCommand(kube.GetResourceKey(obj), "create")
		return "", err
	}
	s.SetLastResourceCommand(kube.GetResourceKey(obj), "create")
	return "created", nil
}
