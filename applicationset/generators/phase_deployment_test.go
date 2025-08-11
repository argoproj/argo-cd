package generators

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestNewPhaseDeploymentProcessor(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	processor := NewPhaseDeploymentProcessor(client)
	
	assert.NotNil(t, processor)
	assert.Equal(t, client, processor.client)
}

func TestGetGeneratorWithPhaseDeployment(t *testing.T) {
	tests := []struct {
		name      string
		generator *argoprojiov1alpha1.ApplicationSetGenerator
		expected  bool
	}{
		{
			name: "generator with phase deployment",
			generator: &argoprojiov1alpha1.ApplicationSetGenerator{
				DeploymentStrategy: &argoprojiov1alpha1.GeneratorDeploymentStrategy{
					Type: PhaseDeploymentType,
				},
			},
			expected: true,
		},
		{
			name: "generator without deployment strategy",
			generator: &argoprojiov1alpha1.ApplicationSetGenerator{
				List: &argoprojiov1alpha1.ListGenerator{},
			},
			expected: false,
		},
		{
			name: "generator with different deployment strategy type",
			generator: &argoprojiov1alpha1.ApplicationSetGenerator{
				DeploymentStrategy: &argoprojiov1alpha1.GeneratorDeploymentStrategy{
					Type: "different",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetGeneratorWithPhaseDeployment(tt.generator)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPhaseDeploymentProcessor_ProcessPhaseDeployment(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	processor := NewPhaseDeploymentProcessor(client)

	tests := []struct {
		name           string
		appSet         *argoprojiov1alpha1.ApplicationSet
		generator      *argoprojiov1alpha1.ApplicationSetGenerator
		generatedParams []map[string]any
		expectedParams  []map[string]any
		expectError    bool
	}{
		{
			name: "no deployment strategy",
			appSet: &argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-appset",
					Namespace: "default",
				},
			},
			generator: &argoprojiov1alpha1.ApplicationSetGenerator{
				List: &argoprojiov1alpha1.ListGenerator{},
			},
			generatedParams: []map[string]any{
				{"cluster": "dev", "name": "app1"},
				{"cluster": "prod", "name": "app2"},
			},
			expectedParams: []map[string]any{
				{"cluster": "dev", "name": "app1"},
				{"cluster": "prod", "name": "app2"},
			},
			expectError: false,
		},
		{
			name: "phase deployment with no phases",
			appSet: &argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-appset",
					Namespace: "default",
				},
			},
			generator: &argoprojiov1alpha1.ApplicationSetGenerator{
				DeploymentStrategy: &argoprojiov1alpha1.GeneratorDeploymentStrategy{
					Type:   PhaseDeploymentType,
					Phases: []argoprojiov1alpha1.GeneratorDeploymentPhase{},
				},
			},
			generatedParams: []map[string]any{
				{"cluster": "dev", "name": "app1"},
			},
			expectedParams: nil,
			expectError:    true,
		},
		{
			name: "phase deployment with cluster targeting",
			appSet: &argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-appset",
					Namespace: "default",
				},
			},
			generator: &argoprojiov1alpha1.ApplicationSetGenerator{
				DeploymentStrategy: &argoprojiov1alpha1.GeneratorDeploymentStrategy{
					Type: PhaseDeploymentType,
					Phases: []argoprojiov1alpha1.GeneratorDeploymentPhase{
						{
							Name: "dev-phase",
							Targets: []argoprojiov1alpha1.GeneratorPhaseTarget{
								{
									Clusters: []string{"dev"},
								},
							},
						},
					},
				},
			},
			generatedParams: []map[string]any{
				{"cluster": "dev", "name": "app1"},
				{"cluster": "prod", "name": "app2"},
			},
			expectedParams: []map[string]any{
				{"cluster": "dev", "name": "app1"},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessPhaseDeployment(context.TODO(), tt.appSet, tt.generator, tt.generatedParams)
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedParams, result)
			}
		})
	}
}

func TestPhaseDeploymentProcessor_filterParamsForPhase(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	processor := NewPhaseDeploymentProcessor(client)

	maxUpdate := intstr.FromInt(1)
	percentage20 := int64(20)
	percentage50 := int64(50)
	
	tests := []struct {
		name           string
		allParams      []map[string]any
		phase          argoprojiov1alpha1.GeneratorDeploymentPhase
		expectedParams []map[string]any
	}{
		{
			name: "no targets - return all params",
			allParams: []map[string]any{
				{"cluster": "dev", "name": "app1"},
				{"cluster": "prod", "name": "app2"},
			},
			phase: argoprojiov1alpha1.GeneratorDeploymentPhase{
				Name: "all-phase",
			},
			expectedParams: []map[string]any{
				{"cluster": "dev", "name": "app1"},
				{"cluster": "prod", "name": "app2"},
			},
		},
		{
			name: "cluster targeting",
			allParams: []map[string]any{
				{"cluster": "dev", "name": "app1"},
				{"cluster": "staging", "name": "app2"},
				{"cluster": "prod", "name": "app3"},
			},
			phase: argoprojiov1alpha1.GeneratorDeploymentPhase{
				Name: "dev-phase",
				Targets: []argoprojiov1alpha1.GeneratorPhaseTarget{
					{
						Clusters: []string{"dev", "staging"},
					},
				},
			},
			expectedParams: []map[string]any{
				{"cluster": "dev", "name": "app1"},
				{"cluster": "staging", "name": "app2"},
			},
		},
		{
			name: "maxUpdate constraint",
			allParams: []map[string]any{
				{"cluster": "dev", "name": "app1"},
				{"cluster": "staging", "name": "app2"},
			},
			phase: argoprojiov1alpha1.GeneratorDeploymentPhase{
				Name: "limited-phase",
				Targets: []argoprojiov1alpha1.GeneratorPhaseTarget{
					{
						Clusters: []string{"dev", "staging"},
					},
				},
				MaxUpdate: &maxUpdate,
			},
			expectedParams: []map[string]any{
				{"cluster": "dev", "name": "app1"},
			},
		},
		{
			name: "value matching",
			allParams: []map[string]any{
				{"environment": "dev", "name": "app1"},
				{"environment": "prod", "name": "app2"},
				{"environment": "dev", "name": "app3"},
			},
			phase: argoprojiov1alpha1.GeneratorDeploymentPhase{
				Name: "dev-env-phase",
				Targets: []argoprojiov1alpha1.GeneratorPhaseTarget{
					{
						Values: map[string]string{
							"environment": "dev",
						},
					},
				},
			},
			expectedParams: []map[string]any{
				{"environment": "dev", "name": "app1"},
				{"environment": "dev", "name": "app3"},
			},
		},
		{
			name: "percentage-based deployment - 20%",
			allParams: []map[string]any{
				{"name": "app1"},
				{"name": "app2"},
				{"name": "app3"},
				{"name": "app4"},
				{"name": "app5"},
			},
			phase: argoprojiov1alpha1.GeneratorDeploymentPhase{
				Name:       "canary-phase",
				Percentage: &percentage20,
			},
			expectedParams: []map[string]any{
				{"name": "app1"}, // 20% of 5 = 1 app
			},
		},
		{
			name: "percentage-based deployment - 50%",
			allParams: []map[string]any{
				{"name": "app1"},
				{"name": "app2"},
				{"name": "app3"},
				{"name": "app4"},
			},
			phase: argoprojiov1alpha1.GeneratorDeploymentPhase{
				Name:       "half-phase",
				Percentage: &percentage50,
			},
			expectedParams: []map[string]any{
				{"name": "app1"},
				{"name": "app2"}, // 50% of 4 = 2 apps
			},
		},
		{
			name: "percentage with maxUpdate constraint",
			allParams: []map[string]any{
				{"name": "app1"},
				{"name": "app2"},
				{"name": "app3"},
				{"name": "app4"},
			},
			phase: argoprojiov1alpha1.GeneratorDeploymentPhase{
				Name:       "limited-percentage",
				Percentage: &percentage50,
				MaxUpdate:  &maxUpdate,
			},
			expectedParams: []map[string]any{
				{"name": "app1"}, // maxUpdate=1 overrides percentage
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.filterParamsForPhase(tt.allParams, tt.phase)
			assert.Equal(t, tt.expectedParams, result)
		})
	}
}

func TestPhaseDeploymentProcessor_evaluateMatchExpression(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	processor := NewPhaseDeploymentProcessor(client)

	tests := []struct {
		name     string
		param    map[string]any
		expr     argoprojiov1alpha1.ApplicationMatchExpression
		expected bool
	}{
		{
			name:  "In operator - match",
			param: map[string]any{"environment": "dev"},
			expr: argoprojiov1alpha1.ApplicationMatchExpression{
				Key:      "environment",
				Operator: "In",
				Values:   []string{"dev", "test"},
			},
			expected: true,
		},
		{
			name:  "In operator - no match",
			param: map[string]any{"environment": "prod"},
			expr: argoprojiov1alpha1.ApplicationMatchExpression{
				Key:      "environment",
				Operator: "In",
				Values:   []string{"dev", "test"},
			},
			expected: false,
		},
		{
			name:  "NotIn operator - match",
			param: map[string]any{"environment": "prod"},
			expr: argoprojiov1alpha1.ApplicationMatchExpression{
				Key:      "environment",
				Operator: "NotIn",
				Values:   []string{"dev", "test"},
			},
			expected: true,
		},
		{
			name:  "Exists operator - field exists",
			param: map[string]any{"environment": "dev"},
			expr: argoprojiov1alpha1.ApplicationMatchExpression{
				Key:      "environment",
				Operator: "Exists",
			},
			expected: true,
		},
		{
			name:  "DoesNotExist operator - field missing",
			param: map[string]any{"cluster": "dev"},
			expr: argoprojiov1alpha1.ApplicationMatchExpression{
				Key:      "environment",
				Operator: "DoesNotExist",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.evaluateMatchExpression(tt.param, tt.expr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPhaseDeploymentProcessor_getCurrentPhase(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	processor := NewPhaseDeploymentProcessor(client)

	tests := []struct {
		name      string
		appSet    *argoprojiov1alpha1.ApplicationSet
		generator *argoprojiov1alpha1.ApplicationSetGenerator
		expected  int
	}{
		{
			name: "no annotations",
			appSet: &argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-appset",
				},
			},
			generator: &argoprojiov1alpha1.ApplicationSetGenerator{
				List: &argoprojiov1alpha1.ListGenerator{},
			},
			expected: 0,
		},
		{
			name: "phase annotation exists",
			appSet: &argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-appset",
					Annotations: map[string]string{
						"applicationset.argoproj.io/phase-list": "2",
					},
				},
			},
			generator: &argoprojiov1alpha1.ApplicationSetGenerator{
				List: &argoprojiov1alpha1.ListGenerator{},
			},
			expected: 2,
		},
		{
			name: "invalid phase annotation",
			appSet: &argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-appset",
					Annotations: map[string]string{
						"applicationset.argoproj.io/phase-list": "invalid",
					},
				},
			},
			generator: &argoprojiov1alpha1.ApplicationSetGenerator{
				List: &argoprojiov1alpha1.ListGenerator{},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.getCurrentPhase(tt.appSet, tt.generator)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPhaseDeploymentProcessor_getGeneratorType(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	processor := NewPhaseDeploymentProcessor(client)

	tests := []struct {
		name      string
		generator *argoprojiov1alpha1.ApplicationSetGenerator
		expected  string
	}{
		{
			name: "list generator",
			generator: &argoprojiov1alpha1.ApplicationSetGenerator{
				List: &argoprojiov1alpha1.ListGenerator{},
			},
			expected: "list",
		},
		{
			name: "clusters generator",
			generator: &argoprojiov1alpha1.ApplicationSetGenerator{
				Clusters: &argoprojiov1alpha1.ClusterGenerator{},
			},
			expected: "clusters",
		},
		{
			name: "git generator",
			generator: &argoprojiov1alpha1.ApplicationSetGenerator{
				Git: &argoprojiov1alpha1.GitGenerator{},
			},
			expected: "git",
		},
		{
			name: "unknown generator",
			generator: &argoprojiov1alpha1.ApplicationSetGenerator{},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.getGeneratorType(tt.generator)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPhaseDeploymentProcessor_GetGeneratorPhaseStatus(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	processor := NewPhaseDeploymentProcessor(client)

	tests := []struct {
		name           string
		appSet         *argoprojiov1alpha1.ApplicationSet
		generator      *argoprojiov1alpha1.ApplicationSetGenerator
		expectedCurrent int
		expectedTotal   int
		expectError     bool
	}{
		{
			name: "no deployment strategy",
			appSet: &argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
			},
			generator: &argoprojiov1alpha1.ApplicationSetGenerator{
				List: &argoprojiov1alpha1.ListGenerator{},
			},
			expectError: true,
		},
		{
			name: "valid phase deployment",
			appSet: &argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						"applicationset.argoproj.io/phase-list": "1",
					},
				},
			},
			generator: &argoprojiov1alpha1.ApplicationSetGenerator{
				List: &argoprojiov1alpha1.ListGenerator{},
				DeploymentStrategy: &argoprojiov1alpha1.GeneratorDeploymentStrategy{
					Type: PhaseDeploymentType,
					Phases: []argoprojiov1alpha1.GeneratorDeploymentPhase{
						{Name: "phase1"},
						{Name: "phase2"},
						{Name: "phase3"},
					},
				},
			},
			expectedCurrent: 1,
			expectedTotal:   3,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current, total, err := processor.GetGeneratorPhaseStatus(tt.appSet, tt.generator)
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCurrent, current)
				assert.Equal(t, tt.expectedTotal, total)
			}
		})
	}
}

func TestGeneratorPhaseStatusToJSON(t *testing.T) {
	tests := []struct {
		name         string
		currentPhase int
		totalPhases  int
		expected     string
	}{
		{
			name:         "in progress",
			currentPhase: 1,
			totalPhases:  3,
			expected:     `{"completed":false,"currentPhase":1,"totalPhases":3}`,
		},
		{
			name:         "completed",
			currentPhase: 3,
			totalPhases:  3,
			expected:     `{"completed":true,"currentPhase":3,"totalPhases":3}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GeneratorPhaseStatusToJSON(tt.currentPhase, tt.totalPhases)
			assert.JSONEq(t, tt.expected, result)
		})
	}
}

func TestPhaseDeploymentProcessor_filterParamsForPercentagePhase(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	processor := NewPhaseDeploymentProcessor(client)

	percentage10 := int64(10)
	percentage30 := int64(30)
	percentage50 := int64(50)
	percentage60 := int64(60)
	maxUpdate := intstr.FromInt(1)

	allParams := []map[string]any{
		{"name": "app1", "environment": "dev"},
		{"name": "app2", "environment": "staging"},
		{"name": "app3", "environment": "dev"},
		{"name": "app4", "environment": "prod"},
		{"name": "app5", "environment": "dev"},
		{"name": "app6", "environment": "staging"},
		{"name": "app7", "environment": "prod"},
		{"name": "app8", "environment": "dev"},
		{"name": "app9", "environment": "staging"},
		{"name": "app10", "environment": "prod"},
	}

	tests := []struct {
		name           string
		phases         []argoprojiov1alpha1.GeneratorDeploymentPhase
		currentPhase   int
		expectedParams []map[string]any
	}{
		{
			name: "first phase - 10%",
			phases: []argoprojiov1alpha1.GeneratorDeploymentPhase{
				{Name: "phase1", Percentage: &percentage10},
				{Name: "phase2", Percentage: &percentage30},
				{Name: "phase3", Percentage: &percentage60},
			},
			currentPhase: 0,
			expectedParams: []map[string]any{
				{"name": "app1", "environment": "dev"}, // 10% of 10 = 1 app
			},
		},
		{
			name: "percentage phase with maxUpdate",
			phases: []argoprojiov1alpha1.GeneratorDeploymentPhase{
				{Name: "phase1", Percentage: &percentage30, MaxUpdate: &maxUpdate},
			},
			currentPhase: 0,
			expectedParams: []map[string]any{
				{"name": "app1", "environment": "dev"}, // maxUpdate=1 overrides percentage
			},
		},
		{
			name: "percentage with cluster filtering",
			phases: []argoprojiov1alpha1.GeneratorDeploymentPhase{
				{
					Name:       "filtered-phase",
					Percentage: &percentage50,
					Targets: []argoprojiov1alpha1.GeneratorPhaseTarget{
						{
							MatchExpressions: []argoprojiov1alpha1.ApplicationMatchExpression{
								{
									Key:      "environment",
									Operator: "In",
									Values:   []string{"dev"},
								},
							},
						},
					},
				},
			},
			currentPhase: 0,
			expectedParams: []map[string]any{
				{"name": "app1", "environment": "dev"},
				{"name": "app3", "environment": "dev"}, // 50% of 4 dev apps = 2 apps
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.filterParamsForPercentagePhase(allParams, tt.phases, tt.currentPhase)
			assert.Equal(t, tt.expectedParams, result)
		})
	}
}

func TestPhaseDeploymentProcessor_isPercentageBasedStrategy(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	processor := NewPhaseDeploymentProcessor(client)

	percentage10 := int64(10)

	tests := []struct {
		name     string
		phases   []argoprojiov1alpha1.GeneratorDeploymentPhase
		expected bool
	}{
		{
			name: "percentage-based strategy",
			phases: []argoprojiov1alpha1.GeneratorDeploymentPhase{
				{Name: "phase1", Percentage: &percentage10},
			},
			expected: true,
		},
		{
			name: "target-based strategy",
			phases: []argoprojiov1alpha1.GeneratorDeploymentPhase{
				{
					Name: "phase1",
					Targets: []argoprojiov1alpha1.GeneratorPhaseTarget{
						{Clusters: []string{"dev"}},
					},
				},
			},
			expected: false,
		},
		{
			name: "mixed strategy",
			phases: []argoprojiov1alpha1.GeneratorDeploymentPhase{
				{Name: "phase1", Percentage: &percentage10},
				{
					Name: "phase2",
					Targets: []argoprojiov1alpha1.GeneratorPhaseTarget{
						{Clusters: []string{"prod"}},
					},
				},
			},
			expected: true, // Any phase with percentage makes it percentage-based
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.isPercentageBasedStrategy(tt.phases)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPhaseDeploymentProcessor_runPhaseHooks(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	processor := NewPhaseDeploymentProcessor(client)
	ctx := context.Background()
	
	appSet := &argoprojiov1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-appset",
			Namespace: "test-ns",
		},
	}

	tests := []struct {
		name      string
		hooks     []argoprojiov1alpha1.GeneratorPhaseHook
		hookType  string
		expectErr bool
	}{
		{
			name:     "no hooks",
			hooks:    []argoprojiov1alpha1.GeneratorPhaseHook{},
			hookType: "pre",
			expectErr: false,
		},
		{
			name: "simple echo command hook",
			hooks: []argoprojiov1alpha1.GeneratorPhaseHook{
				{
					Name: "test-command",
					Type: "command",
					Command: &argoprojiov1alpha1.GeneratorPhaseCheckCommand{
						Command: []string{"echo", "test"},
					},
					FailurePolicy: "fail",
				},
			},
			hookType:  "pre",
			expectErr: false,
		},
		{
			name: "failing command hook with ignore policy",
			hooks: []argoprojiov1alpha1.GeneratorPhaseHook{
				{
					Name: "failing-command",
					Type: "command",
					Command: &argoprojiov1alpha1.GeneratorPhaseCheckCommand{
						Command: []string{"false"},
					},
					FailurePolicy: "ignore",
				},
			},
			hookType:  "pre",
			expectErr: false,
		},
		{
			name: "failing command hook with fail policy",
			hooks: []argoprojiov1alpha1.GeneratorPhaseHook{
				{
					Name: "failing-command",
					Type: "command",
					Command: &argoprojiov1alpha1.GeneratorPhaseCheckCommand{
						Command: []string{"false"},
					},
					FailurePolicy: "fail",
				},
			},
			hookType:  "pre",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := log.WithField("test", "runPhaseHooks")
			err := processor.runPhaseHooks(ctx, appSet, tt.hooks, tt.hookType, logger)
			
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPhaseDeploymentProcessor_runCommandHook(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	processor := NewPhaseDeploymentProcessor(client)
	ctx := context.Background()
	
	appSet := &argoprojiov1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-appset",
			Namespace: "test-ns",
		},
	}

	tests := []struct {
		name      string
		hook      argoprojiov1alpha1.GeneratorPhaseHook
		hookType  string
		expectErr bool
	}{
		{
			name: "successful echo command",
			hook: argoprojiov1alpha1.GeneratorPhaseHook{
				Name: "echo-test",
				Type: "command",
				Command: &argoprojiov1alpha1.GeneratorPhaseCheckCommand{
					Command: []string{"echo", "test"},
				},
			},
			hookType:  "pre",
			expectErr: false,
		},
		{
			name: "missing command field",
			hook: argoprojiov1alpha1.GeneratorPhaseHook{
				Name: "no-command",
				Type: "command",
			},
			hookType:  "pre",
			expectErr: true,
		},
		{
			name: "empty command array",
			hook: argoprojiov1alpha1.GeneratorPhaseHook{
				Name: "empty-command",
				Type: "command",
				Command: &argoprojiov1alpha1.GeneratorPhaseCheckCommand{
					Command: []string{},
				},
			},
			hookType:  "pre",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := log.WithField("test", "runCommandHook")
			err := processor.runCommandHook(ctx, appSet, tt.hook, tt.hookType, logger)
			
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}