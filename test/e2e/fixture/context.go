package fixture

import (
	"fmt"
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/v3/util/rand"

	"github.com/argoproj/argo-cd/v3/util/errors"
)

// TestContext defines the interface for test-specific state that enables parallel test execution.
// All fixture Context types should implement this interface by embedding TestState.
type TestContext interface {
	// SetName sets the DNS-friendly name for this context
	SetName(name string)
	// GetName returns the DNS-friendly name for this context
	GetName() string
	// DeploymentNamespace returns the namespace where test resources are deployed
	DeploymentNamespace() string
	// ID returns the unique identifier for this test run
	ID() string
	// ShortID returns the short unique identifier suffix for this test run
	ShortID() string
	// Token returns the authentication token for API calls
	Token() string
	// SetToken sets the authentication token
	SetToken(token string)
	// T returns the testing.T instance for this test
	T() *testing.T
}

// TestState holds test-specific variables that were previously global.
// Embed this in Context structs to enable parallel test execution.
type TestState struct {
	t                   *testing.T
	id                  string
	shortId             string
	name                string
	deploymentNamespace string
	token               string
}

// NewTestState creates a new TestState with unique identifiers for this test run.
// This generates fresh id, name, and deploymentNamespace values.
func NewTestState(t *testing.T) *TestState {
	t.Helper()
	randString, err := rand.String(5)
	errors.CheckError(err)
	shortId := strings.ToLower(randString)

	return &TestState{
		t:                   t,
		token:               token, // Initialize with current global token
		id:                  fmt.Sprintf("%s-%s", t.Name(), shortId),
		shortId:             shortId,
		name:                DnsFriendly(t.Name(), "-"+shortId),
		deploymentNamespace: DnsFriendly("argocd-e2e-"+t.Name(), "-"+shortId),
	}
}

// NewTestStateFromContext creates a TestState from an existing TestContext.
// This allows GivenWithSameState functions to work across different Context types.
func NewTestStateFromContext(ctx TestContext) *TestState {
	return &TestState{
		t:                   ctx.T(),
		id:                  ctx.ID(),
		shortId:             ctx.ShortID(),
		name:                ctx.GetName(),
		deploymentNamespace: ctx.DeploymentNamespace(),
		token:               ctx.Token(),
	}
}

// Name sets the DNS-friendly name for this context
func (s *TestState) SetName(name string) {
	suffix := "-" + s.shortId
	s.name = DnsFriendly(strings.TrimSuffix(name, suffix), suffix)
}

// GetName returns the DNS-friendly name for this context
func (s *TestState) GetName() string {
	return s.name
}

// DeploymentNamespace returns the namespace where test resources are deployed
func (s *TestState) DeploymentNamespace() string {
	return s.deploymentNamespace
}

// ID returns the unique identifier for this test run
func (s *TestState) ID() string {
	return s.id
}

// ShortID returns the short unique identifier suffix for this test run
func (s *TestState) ShortID() string {
	return s.shortId
}

// Token returns the authentication token for API calls
func (s *TestState) Token() string {
	return s.token
}

// SetToken sets the authentication token
func (s *TestState) SetToken(token string) {
	s.token = token
}

// T returns the testing.T instance for this test
func (s *TestState) T() *testing.T {
	return s.t
}
