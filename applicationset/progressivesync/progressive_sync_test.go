package progressivesync

import (
	"context"
	"testing"
	"time"

	appfake "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/fake"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestBuildAppDependencyList(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	for _, cc := range []struct {
		name                     string
		appSet                   v1alpha1.ApplicationSet
		apps                     []v1alpha1.Application
		expectedList             [][]string
		expectedStepMap          map[string]int
		expectedValidationIssues *ValidationIssues
	}{
		{
			name: "handles an empty set of applications and no strategy",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{},
			},
			apps:                     []v1alpha1.Application{},
			expectedList:             [][]string{},
			expectedStepMap:          map[string]int{},
			expectedValidationIssues: &ValidationIssues{},
		},
		{
			name: "handles an empty set of applications and ignores AllAtOnce strategy",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "AllAtOnce",
					},
				},
			},
			apps:                     []v1alpha1.Application{},
			expectedList:             [][]string{},
			expectedStepMap:          map[string]int{},
			expectedValidationIssues: &ValidationIssues{},
		},
		{
			name: "handles an empty set of applications with good 'In' selectors",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "In",
											Values: []string{
												"dev",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{},
			expectedList: [][]string{
				{},
			},
			expectedStepMap: map[string]int{},
			expectedValidationIssues: &ValidationIssues{
				EmptySteps: []int{0},
			},
		},
		{
			name: "handles selecting 1 application with 1 'In' selector",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "In",
											Values: []string{
												"dev",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					Name: "app-dev",
					Labels: map[string]string{
						"env": "dev",
					},
				},
			},
			expectedList: [][]string{
				{"app-dev"},
			},
			expectedStepMap: map[string]int{
				"app-dev": 0,
			},
			expectedValidationIssues: &ValidationIssues{},
		},
		{
			name: "handles 'In' selectors that select no applications",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "In",
											Values: []string{
												"dev",
											},
										},
									},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "In",
											Values: []string{
												"qa",
											},
										},
									},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "In",
											Values: []string{
												"prod",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					Name: "app-qa",
					Labels: map[string]string{
						"env": "qa",
					},
				},
				{
					Name: "app-prod",
					Labels: map[string]string{
						"env": "prod",
					},
				},
			},
			expectedList: [][]string{
				{},
				{"app-qa"},
				{"app-prod"},
			},
			expectedStepMap: map[string]int{
				"app-qa":   1,
				"app-prod": 2,
			},
			expectedValidationIssues: &ValidationIssues{
				EmptySteps: []int{0},
			},
		},
		{
			name: "multiple 'In' selectors in the same matchExpression only select Applications that match all selectors",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "region",
											Operator: "In",
											Values: []string{
												"us-east-2",
											},
										},
										{
											Key:      "env",
											Operator: "In",
											Values: []string{
												"qa",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					Name: "app-qa1",
					Labels: map[string]string{
						"env": "qa",
					},
				},
				{
					Name: "app-qa2",
					Labels: map[string]string{
						"env":    "qa",
						"region": "us-east-2",
					},
				},
			},
			expectedList: [][]string{
				{"app-qa2"},
			},
			expectedStepMap: map[string]int{
				"app-qa2": 0,
			},
			// TO-DO: app-qa1 is not selected by any step but is generated - should be validationIssue
			expectedValidationIssues: &ValidationIssues{},
		},
		{
			name: "multiple values in the same 'In' matchExpression can match on any value",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "In",
											Values: []string{
												"qa",
												"prod",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					Name: "app-dev",
					Labels: map[string]string{
						"env": "dev",
					},
				},
				{
					Name: "app-qa",
					Labels: map[string]string{
						"env": "qa",
					},
				},
				{
					Name: "app-prod",
					Labels: map[string]string{
						"env":    "prod",
						"region": "us-east-2",
					},
				},
			},
			expectedList: [][]string{
				{"app-qa", "app-prod"},
			},
			expectedStepMap: map[string]int{
				"app-qa":   0,
				"app-prod": 0,
			},
			expectedValidationIssues: &ValidationIssues{},
		},
		{
			name: "handles an empty set of applications with good 'NotIn' selectors",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "NotIn",
											Values: []string{
												"dev",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{},
			expectedList: [][]string{
				{},
			},
			expectedStepMap: map[string]int{},
			expectedValidationIssues: &ValidationIssues{
				EmptySteps: []int{0},
			},
		},
		{
			name: "selects 1 application with 1 'NotIn' selector",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "NotIn",
											Values: []string{
												"qa",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					Name: "app-dev",
					Labels: map[string]string{
						"env": "dev",
					},
				},
			},
			expectedList: [][]string{
				{"app-dev"},
			},
			expectedStepMap: map[string]int{
				"app-dev": 0,
			},
			expectedValidationIssues: &ValidationIssues{},
		},
		{
			name: "'NotIn' selectors that select no applications",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "NotIn",
											Values: []string{
												"qa", "prod",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					Name: "app-qa",
					Labels: map[string]string{
						"env": "qa",
					},
				},
				{
					Name: "app-prod",
					Labels: map[string]string{
						"env": "prod",
					},
				},
			},
			expectedList:    [][]string{{}},
			expectedStepMap: map[string]int{},
			expectedValidationIssues: &ValidationIssues{
				EmptySteps: []int{0},
			},
		},
		{
			name: "multiple 'NotIn' selectors remove Applications with mising labels on any match",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "region",
											Operator: "NotIn",
											Values: []string{
												"us-east-2",
											},
										},
										{
											Key:      "env",
											Operator: "NotIn",
											Values: []string{
												"qa",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					Name: "app-qa1",
					Labels: map[string]string{
						"env": "qa",
					},
				},
				{
					Name: "app-qa2",
					Labels: map[string]string{
						"env":    "qa",
						"region": "us-east-2",
					},
				},
			},
			expectedList: [][]string{
				{},
			},
			expectedStepMap: map[string]int{},
			expectedValidationIssues: &ValidationIssues{
				EmptySteps: []int{0},
			},
		},
		{
			name: "multiple 'NotIn' selectors filter all matching Applications",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "region",
											Operator: "NotIn",
											Values: []string{
												"us-east-2",
											},
										},
										{
											Key:      "env",
											Operator: "NotIn",
											Values: []string{
												"qa",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					Name: "app-qa1",
					Labels: map[string]string{
						"env":    "qa",
						"region": "us-east-1",
					},
				},
				{
					Name: "app-qa2",
					Labels: map[string]string{
						"env":    "qa",
						"region": "us-east-2",
					},
				},
				{
					Name: "app-prod1",
					Labels: map[string]string{
						"env":    "prod",
						"region": "us-east-1",
					},
				},
				{
					Name: "app-prod2",
					Labels: map[string]string{
						"env":    "prod",
						"region": "us-east-2",
					},
				},
			},
			expectedList: [][]string{
				{"app-prod1"},
			},
			expectedStepMap: map[string]int{
				"app-prod1": 0,
			},
			expectedValidationIssues: &ValidationIssues{},
		},
		{
			name: "multiple values in the same 'NotIn' matchExpression exclude a match from any value",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "NotIn",
											Values: []string{
												"qa",
												"prod",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					Name: "app-dev",
					Labels: map[string]string{
						"env": "dev",
					},
				},
				{
					Name: "app-qa",
					Labels: map[string]string{
						"env": "qa",
					},
				},
				{
					Name: "app-prod",
					Labels: map[string]string{
						"env":    "prod",
						"region": "us-east-2",
					},
				},
			},
			expectedList: [][]string{
				{"app-dev"},
			},
			expectedStepMap: map[string]int{
				"app-dev": 0,
			},
			expectedValidationIssues: &ValidationIssues{},
		},
		{
			name: "in a mix of 'In' and 'NotIn' selectors, 'NotIn' takes precedence",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "In",
											Values: []string{
												"qa",
												"prod",
											},
										},
										{
											Key:      "region",
											Operator: "NotIn",
											Values: []string{
												"us-west-2",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					Name: "app-dev",
					Labels: map[string]string{
						"env": "dev",
					},
				},
				{
					Name: "app-qa1",
					Labels: map[string]string{
						"env":    "qa",
						"region": "us-west-2",
					},
				},
				{
					Name: "app-qa2",
					Labels: map[string]string{
						"env":    "qa",
						"region": "us-east-2",
					},
				},
			},
			expectedList: [][]string{
				{"app-qa2"},
			},
			expectedStepMap: map[string]int{
				"app-qa2": 0,
			},
			expectedValidationIssues: &ValidationIssues{},
		},
		{
			name: "app selected in multiple steps is captured as validation issue",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app-matches-multiple-steps",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "In",
											Values: []string{
												"dev",
											},
										},
									},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "region",
											Operator: "In",
											Values: []string{
												"us-west-2",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					Name: "app-dev",
					Labels: map[string]string{
						"env":    "dev",
						"region": "us-west-2",
					},
				},
				{
					Name: "app-qa",
					Labels: map[string]string{
						"region": "us-west-2",
					},
				},
			},
			expectedList: [][]string{
				{"app-dev"},
				{"app-dev", "app-qa"},
			},
			expectedStepMap: map[string]int{
				"app-dev": 0,
				"app-qa":  1,
			},
			expectedValidationIssues: &ValidationIssues{
				DuplicateAppSelections: map[string][]int{
					"app-dev": {0, 1},
				},
			},
		},
		{
			name: "Invalid Operator in MatchExpression is captured as validationIssue",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-match-expression",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "Invalid",
											Values: []string{
												"dev",
											},
										},
									},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "env",
											Operator: "In",
											Values: []string{
												"dev",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			apps: []v1alpha1.Application{
				{
					Name: "app-dev",
					Labels: map[string]string{
						"env": "dev",
					},
				},
			},
			expectedList: [][]string{
				{},
				{"app-dev"},
			},
			expectedStepMap: map[string]int{
				"app-dev": 1,
			},
			expectedValidationIssues: &ValidationIssues{
				EmptySteps: []int{0},
				InvalidMatchExpressions: []InvalidMatchExpression{
					{
						StepIndex: 0,
						Operator:  "Invalid",
					},
				},
			},
		},
	} {
		t.Run(cc.name, func(t *testing.T) {
			t.Parallel()
			appDependencyList, appStepMap, validationIssues := buildAppDependencyList(log.NewEntry(log.StandardLogger()), cc.appSet, cc.apps)
			assert.Equal(t, cc.expectedList, appDependencyList, "expected appDependencyList did not match actual")
			assert.Equal(t, cc.expectedStepMap, appStepMap, "expected appStepMap did not match actual")
			assert.Equal(t, cc.expectedValidationIssues, validationIssues, "expected validationIssues did not match actual")
		})
	}
}

func TestGetAppsToSync(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	for _, cc := range []struct {
		name              string
		appSet            v1alpha1.ApplicationSet
		currentApps       []v1alpha1.Application
		appDependencyList [][]string
		expectedMap       map[string]bool
	}{
		{
			name: "handles an empty app dependency list",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
			},
			appDependencyList: [][]string{},
			expectedMap:       map[string]bool{},
		},
		{
			name: "handles missing applications with statuses",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Status:      v1alpha1.ProgressiveSyncHealthy,
						},
						{
							Application: "app2",
							Status:      v1alpha1.ProgressiveSyncHealthy,
						},
					},
				},
			},
			currentApps: []v1alpha1.Application{
				{Name: "app2"},
			},
			appDependencyList: [][]string{
				{"app1"},
				{"app2"},
			},
			expectedMap: map[string]bool{
				"app1": true,
			},
		},
		{
			name: "handles new applications with no statuses",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
			},
			currentApps: []v1alpha1.Application{
				{Name: "app1"},
				{Name: "app2"},
			},
			appDependencyList: [][]string{
				{"app1"},
				{"app2"},
			},
			expectedMap: map[string]bool{
				"app1": true,
			},
		},
		{
			name: "handles an empty step as completed",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
			},
			currentApps: []v1alpha1.Application{
				{Name: "app1"},
				{Name: "app2"},
			},
			appDependencyList: [][]string{
				{},
				{"app1", "app2"},
			},
			expectedMap: map[string]bool{
				"app1": true,
				"app2": true,
			},
		},
		{
			name: "handles healthy steps",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Status:      v1alpha1.ProgressiveSyncHealthy,
						},
						{
							Application: "app2",
							Status:      v1alpha1.ProgressiveSyncHealthy,
						},
						{
							Application: "app3",
							Status:      v1alpha1.ProgressiveSyncHealthy,
						},
						{
							Application: "app4",
							Status:      v1alpha1.ProgressiveSyncHealthy,
						},
					},
				},
			},
			currentApps: []v1alpha1.Application{
				{Name: "app1"},
				{Name: "app2"},
				{Name: "app3"},
				{Name: "app4"},
			},
			appDependencyList: [][]string{
				{"app1", "app2"},
				{"app3", "app4"},
			},
			expectedMap: map[string]bool{
				"app1": true,
				"app2": true,
				"app3": true,
				"app4": true,
			},
		},
		{
			name: "do not consider waiting steps as completed",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Status:      v1alpha1.ProgressiveSyncWaiting,
						},
						{
							Application: "app2",
							Status:      v1alpha1.ProgressiveSyncHealthy,
						},
					},
				},
			},
			currentApps: []v1alpha1.Application{
				{Name: "app1"},
				{Name: "app2"},
			},
			appDependencyList: [][]string{
				{"app1"},
				{"app2"},
			},
			expectedMap: map[string]bool{
				"app1": true,
			},
		},
		{
			name: "do not consider pending steps as completed",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Status:      v1alpha1.ProgressiveSyncPending,
						},
						{
							Application: "app2",
							Status:      v1alpha1.ProgressiveSyncHealthy,
						},
					},
				},
			},
			currentApps: []v1alpha1.Application{
				{Name: "app1"},
				{Name: "app2"},
			},
			appDependencyList: [][]string{
				{"app1"},
				{"app2"},
			},
			expectedMap: map[string]bool{
				"app1": true,
			},
		},
		{
			name: "do not consider progressing steps as completed",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "app1",
							Status:      v1alpha1.ProgressiveSyncProgressing,
						},
						{
							Application: "app2",
							Status:      v1alpha1.ProgressiveSyncHealthy,
						},
					},
				},
			},
			currentApps: []v1alpha1.Application{
				{Name: "app1"},
				{Name: "app2"},
			},
			appDependencyList: [][]string{
				{"app1"},
				{"app2"},
			},
			expectedMap: map[string]bool{
				"app1": true,
			},
		},
		{
			name: "Ignores applications not selected",
			appSet: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{},
								},
							},
						},
					},
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application: "old_app",
							Status:      v1alpha1.ProgressiveSyncProgressing,
						},
						{
							Application: "app1",
							Status:      v1alpha1.ProgressiveSyncHealthy,
						},
						{
							Application: "app2",
							Status:      v1alpha1.ProgressiveSyncHealthy,
						},
					},
				},
			},
			currentApps: []v1alpha1.Application{
				{Name: "old_app"},
				{Name: "app1"},
				{Name: "app2"},
			},
			appDependencyList: [][]string{
				{"app1"},
				{"app2"},
			},
			expectedMap: map[string]bool{
				"app1": true,
				"app2": true,
			},
		},
	} {
		t.Run(cc.name, func(t *testing.T) {
			t.Parallel()
			appsToSync := getAppsToSync(cc.appSet, cc.appDependencyList, cc.currentApps)
			assert.Equal(t, cc.expectedMap, appsToSync, "expected map did not match actual")
		})
	}
}

func TestIsRollingSyncStrategy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		appset   *v1alpha1.ApplicationSet
		expected bool
	}{
		{
			name: "RollingSync strategy is explicitly set",
			appset: &v1alpha1.ApplicationSet{
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "AllAtOnce strategy is explicitly set",
			appset: &v1alpha1.ApplicationSet{
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "AllAtOnce",
					},
				},
			},
			expected: false,
		},
		{
			name: "Strategy is empty",
			appset: &v1alpha1.ApplicationSet{
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{},
				},
			},
			expected: false,
		},
		{
			name: "Strategy is nil",
			appset: &v1alpha1.ApplicationSet{
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: nil,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := IsRollingSyncStrategy(tt.appset)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSyncApplication(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		input     v1alpha1.Application
		prune     bool
		revisions []string
		expected  v1alpha1.Application
	}{
		{
			name: "Default retry limit with no SyncPolicy",
			input: v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{},
			},
			prune: false,
			expected: v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{},
				Operation: &v1alpha1.Operation{
					InitiatedBy: v1alpha1.OperationInitiator{
						Username:  "applicationset-controller",
						Automated: true,
					},
					Info: []*v1alpha1.Info{
						{
							Name:  "Reason",
							Value: "ApplicationSet RollingSync triggered a sync of this Application resource",
						},
					},
					Sync: &v1alpha1.SyncOperation{
						Prune: false,
					},
					Retry: v1alpha1.RetryStrategy{
						Limit: 5,
					},
				},
			},
		},
		{
			name: "Retry and SyncOptions from SyncPolicy are applied",
			input: v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{
					SyncPolicy: &v1alpha1.SyncPolicy{
						Retry: &v1alpha1.RetryStrategy{
							Limit: 10,
						},
						SyncOptions: []string{"CreateNamespace=true"},
					},
				},
			},
			prune: true,
			expected: v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{
					SyncPolicy: &v1alpha1.SyncPolicy{
						Retry: &v1alpha1.RetryStrategy{
							Limit: 10,
						},
						SyncOptions: []string{"CreateNamespace=true"},
					},
				},
				Operation: &v1alpha1.Operation{
					InitiatedBy: v1alpha1.OperationInitiator{
						Username:  "applicationset-controller",
						Automated: true,
					},
					Info: []*v1alpha1.Info{
						{
							Name:  "Reason",
							Value: "ApplicationSet RollingSync triggered a sync of this Application resource",
						},
					},
					Sync: &v1alpha1.SyncOperation{
						SyncOptions: []string{"CreateNamespace=true"},
						Prune:       true,
					},
					Retry: v1alpha1.RetryStrategy{
						Limit: 10,
					},
				},
			},
		},
		{
			name: "single-source application pins Sync.Revision from the recorded target revision",
			input: v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{
					Source: &v1alpha1.ApplicationSource{RepoURL: "https://example.com/repo.git"},
				},
			},
			prune:     false,
			revisions: []string{"abc123"},
			expected: v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{
					Source: &v1alpha1.ApplicationSource{RepoURL: "https://example.com/repo.git"},
				},
				Operation: &v1alpha1.Operation{
					InitiatedBy: v1alpha1.OperationInitiator{
						Username:  "applicationset-controller",
						Automated: true,
					},
					Info: []*v1alpha1.Info{
						{
							Name:  "Reason",
							Value: "ApplicationSet RollingSync triggered a sync of this Application resource",
						},
					},
					Sync: &v1alpha1.SyncOperation{
						Revision: "abc123",
						Prune:    false,
					},
					Retry: v1alpha1.RetryStrategy{
						Limit: 5,
					},
				},
			},
		},
		{
			name: "multi-source application pins Sync.Revisions from the recorded target revisions",
			input: v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{
					Sources: v1alpha1.ApplicationSources{
						{RepoURL: "https://example.com/repo1.git"},
						{RepoURL: "https://example.com/repo2.git"},
					},
				},
			},
			prune:     false,
			revisions: []string{"abc123", "def456"},
			expected: v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{
					Sources: v1alpha1.ApplicationSources{
						{RepoURL: "https://example.com/repo1.git"},
						{RepoURL: "https://example.com/repo2.git"},
					},
				},
				Operation: &v1alpha1.Operation{
					InitiatedBy: v1alpha1.OperationInitiator{
						Username:  "applicationset-controller",
						Automated: true,
					},
					Info: []*v1alpha1.Info{
						{
							Name:  "Reason",
							Value: "ApplicationSet RollingSync triggered a sync of this Application resource",
						},
					},
					Sync: &v1alpha1.SyncOperation{
						Revisions: []string{"abc123", "def456"},
						Prune:     false,
					},
					Retry: v1alpha1.RetryStrategy{
						Limit: 5,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := syncApplication(tt.input, tt.prune, tt.revisions)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsRollingSyncDeletionReversed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		appset   *v1alpha1.ApplicationSet
		expected bool
	}{
		{
			name: "Deletion Order on strategy is set as Reverse",
			appset: &v1alpha1.ApplicationSet{
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "environment",
											Operator: "In",
											Values: []string{
												"dev",
											},
										},
									},
								},
								{
									MatchExpressions: []v1alpha1.ApplicationMatchExpression{
										{
											Key:      "environment",
											Operator: "In",
											Values: []string{
												"staging",
											},
										},
									},
								},
							},
						},
						DeletionOrder: ReverseDeletionOrder,
					},
				},
			},
			expected: true,
		},
		{
			name: "Deletion Order on strategy is set as AllAtOnce",
			appset: &v1alpha1.ApplicationSet{
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{},
						},
						DeletionOrder: AllAtOnceDeletionOrder,
					},
				},
			},
			expected: false,
		},
		{
			name: "Deletion Order on strategy is set as Reverse but no steps in RollingSync",
			appset: &v1alpha1.ApplicationSet{
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "RollingSync",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{},
						},
						DeletionOrder: ReverseDeletionOrder,
					},
				},
			},
			expected: false,
		},
		{
			name: "Deletion Order on strategy is set as Reverse, but AllAtOnce is explicitly set",
			appset: &v1alpha1.ApplicationSet{
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{
						Type: "AllAtOnce",
						RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
							Steps: []v1alpha1.ApplicationSetRolloutStep{},
						},
						DeletionOrder: ReverseDeletionOrder,
					},
				},
			},
			expected: false,
		},
		{
			name: "Strategy is Nil",
			appset: &v1alpha1.ApplicationSet{
				Spec: v1alpha1.ApplicationSetSpec{
					Strategy: &v1alpha1.ApplicationSetStrategy{},
				},
			},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := IsDeletionOrderReversed(tt.appset)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetLatestWaitingTransitionTimeOfAppset(t *testing.T) {
	now := metav1.Now()
	earlierTime := metav1.NewTime(now.Add(-5 * time.Minute))
	laterTime := metav1.NewTime(now.Add(-2 * time.Minute))

	tests := []struct {
		name     string
		appset   *v1alpha1.ApplicationSet
		expected *metav1.Time
	}{
		{
			name: "no applications in waiting state",
			appset: &v1alpha1.ApplicationSet{
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application:        "app1",
							Status:             v1alpha1.ProgressiveSyncHealthy,
							LastTransitionTime: &now,
							Message:            "Application resource became Healthy",
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "brand new application in waiting state (no pending changes message) returns nil",
			appset: &v1alpha1.ApplicationSet{
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application:        "app1",
							Status:             v1alpha1.ProgressiveSyncWaiting,
							LastTransitionTime: &now,
							Message:            "No Application status found, defaulting status to Waiting",
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "single application with pending changes returns its LastTransitionTime",
			appset: &v1alpha1.ApplicationSet{
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application:        "app1",
							Status:             v1alpha1.ProgressiveSyncWaiting,
							LastTransitionTime: &now,
							Message:            "Application has pending changes, setting status to Waiting",
							TargetRevisions:    []string{"new-revision"},
						},
					},
				},
			},
			expected: &now,
		},
		{
			name: "multiple waiting apps with pending changes returns latest LastTransitionTime",
			appset: &v1alpha1.ApplicationSet{
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application:        "app1",
							Status:             v1alpha1.ProgressiveSyncWaiting,
							LastTransitionTime: &laterTime,
							Message:            "Application has pending changes, setting status to Waiting",
							TargetRevisions:    []string{"new-rev"},
						},
						{
							Application:        "app2",
							Status:             v1alpha1.ProgressiveSyncWaiting,
							LastTransitionTime: &earlierTime,
							Message:            "Application has pending changes, setting status to Waiting",
							TargetRevisions:    []string{"new-rev"},
						},
						{
							Application:        "app3",
							Status:             v1alpha1.ProgressiveSyncHealthy,
							LastTransitionTime: &now,
							Message:            "Healthy",
						},
					},
				},
			},
			expected: &laterTime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetLatestWaitingTransitionTimeOfAppset(tt.appset)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.Time, result.Time)
			}
		})
	}
}

func TestCheckAllApplicationsReconciled(t *testing.T) {
	now := metav1.Now()
	before := metav1.NewTime(now.Add(-5 * time.Minute))
	after := metav1.NewTime(now.Add(5 * time.Minute))

	tests := []struct {
		name                      string
		applications              []v1alpha1.Application
		sinceTime                 *metav1.Time
		expected                  bool
		expectedAppsNeedReconcile []v1alpha1.Application
		updatedAppStatus          []v1alpha1.ApplicationSetApplicationStatus
	}{
		{
			name:                      "nil sinceTime returns true",
			applications:              []v1alpha1.Application{},
			sinceTime:                 nil,
			expected:                  true,
			expectedAppsNeedReconcile: nil,
			updatedAppStatus:          []v1alpha1.ApplicationSetApplicationStatus{},
		},
		{
			name: "all applications reconciled after sinceTime with matching revisions",
			applications: []v1alpha1.Application{
				{
					Name: "app1",
					Status: v1alpha1.ApplicationStatus{
						ReconciledAt: &after,
						Sync: v1alpha1.SyncStatus{
							Revision: "abc123",
						},
					},
				},
				{
					Name: "app2",
					Status: v1alpha1.ApplicationStatus{
						ReconciledAt: &after,
						Sync: v1alpha1.SyncStatus{
							Revision: "def456",
						},
					},
				},
			},
			sinceTime:                 &now,
			expected:                  true,
			expectedAppsNeedReconcile: nil,
			updatedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					TargetRevisions: []string{"abc123"},
				},
				{
					Application:     "app2",
					TargetRevisions: []string{"def456"},
				},
			},
		},
		{
			name: "If application reconciledAt after latest transition time, function returns true even if has refreshAnnotation",
			applications: []v1alpha1.Application{
				{
					Name: "app1",
					Annotations: map[string]string{
						v1alpha1.AnnotationKeyRefresh: string(v1alpha1.RefreshTypeNormal),
					},
					Status: v1alpha1.ApplicationStatus{
						ReconciledAt: &after,
					},
				},
			},
			sinceTime:                 &now,
			expected:                  true,
			expectedAppsNeedReconcile: nil, // does not add refresh
			updatedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					TargetRevisions: []string{"abc123"},
				},
			},
		},
		{
			name: "application without ReconciledAt not reconciled",
			applications: []v1alpha1.Application{
				{
					Name: "app1",
					Status: v1alpha1.ApplicationStatus{
						ReconciledAt: nil,
					},
				},
			},
			sinceTime: &now,
			expected:  false,
			expectedAppsNeedReconcile: []v1alpha1.Application{
				{
					Name: "app1",
					Status: v1alpha1.ApplicationStatus{
						ReconciledAt: nil,
					},
				},
			},
			updatedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					TargetRevisions: []string{"abc123"},
				},
			},
		},
		{
			name: "application reconciled before sinceTime not reconciled",
			applications: []v1alpha1.Application{
				{
					Name: "app1",
					Status: v1alpha1.ApplicationStatus{
						ReconciledAt: &before,
					},
				},
			},
			sinceTime: &now,
			expected:  false,
			expectedAppsNeedReconcile: []v1alpha1.Application{
				{
					Name: "app1",
					Status: v1alpha1.ApplicationStatus{
						ReconciledAt: &before,
					},
				},
			},
			updatedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					TargetRevisions: []string{"abc123"},
				},
			},
		},
		{
			name: "mixed reconciliation states returns false",
			applications: []v1alpha1.Application{
				{
					Name: "app1",
					Status: v1alpha1.ApplicationStatus{
						ReconciledAt: &after,
						Sync: v1alpha1.SyncStatus{
							Revision: "abc123",
						},
					},
				},
				{
					Name: "app2",
					Status: v1alpha1.ApplicationStatus{
						ReconciledAt: &before,
						Sync: v1alpha1.SyncStatus{
							Revision: "def456",
						},
					},
				},
			},
			sinceTime: &now,
			expected:  false,
			expectedAppsNeedReconcile: []v1alpha1.Application{
				{
					Name: "app2",
					Status: v1alpha1.ApplicationStatus{
						ReconciledAt: &before,
						Sync: v1alpha1.SyncStatus{
							Revision: "def456",
						},
					},
				},
			},
			updatedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					TargetRevisions: []string{"abc123"},
				},
				{
					Application:     "app2",
					TargetRevisions: []string{"def456"},
				},
			},
		},
		{
			name: "application already has refreshAnnotation, but hasn't been reconciled yet, returns false doesn't add annotation again",
			applications: []v1alpha1.Application{
				{
					Name: "app1",
					Annotations: map[string]string{
						v1alpha1.AnnotationKeyRefresh: string(v1alpha1.RefreshTypeNormal),
					},
					Status: v1alpha1.ApplicationStatus{
						ReconciledAt: &before,
						Sync: v1alpha1.SyncStatus{
							Revision: "abc123",
						},
					},
				},
			},
			sinceTime:                 &now,
			expected:                  false,
			expectedAppsNeedReconcile: nil,
			updatedAppStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app1",
					TargetRevisions: []string{"def123"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, appsNeedReconcile := checkAllApplicationsReconciled(tt.applications, log.NewEntry(log.StandardLogger()), tt.sinceTime)
			assert.Equal(t, tt.expected, result)
			assert.ElementsMatch(t, tt.expectedAppsNeedReconcile, appsNeedReconcile)
		})
	}
}

func TestEnsureApplicationsReconciled(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	now := metav1.Now()
	before := metav1.NewTime(now.Add(-5 * time.Minute))
	after := metav1.NewTime(now.Add(5 * time.Minute))

	tests := []struct {
		name                 string
		appset               v1alpha1.ApplicationSet
		applications         []v1alpha1.Application
		expectedReconciled   bool
		expectError          bool
		latestTransitionTime *metav1.Time
	}{
		{
			name: "no applications in waiting state returns true",
			appset: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-appset",
					Namespace: "argocd",
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application:        "app1",
							Status:             v1alpha1.ProgressiveSyncHealthy,
							LastTransitionTime: &now,
						},
					},
				},
			},
			applications: []v1alpha1.Application{
				{
					Name:      "app1",
					Namespace: "argocd",
					Status: v1alpha1.ApplicationStatus{
						ReconciledAt: &after,
					},
				},
			},
			expectedReconciled:   true,
			expectError:          false,
			latestTransitionTime: nil,
		},
		{
			name: "application in waiting and has refresh annotation, allReconciled returns true if reconciledAt after latestTransitionTime",
			appset: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-appset",
					Namespace: "argocd",
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application:        "app1",
							Status:             v1alpha1.ProgressiveSyncWaiting,
							LastTransitionTime: &before,
							Message:            "pending changes",
						},
					},
				},
			},
			applications: []v1alpha1.Application{
				{
					Name:      "app1",
					Namespace: "argocd",
					Annotations: map[string]string{
						v1alpha1.AnnotationKeyRefresh: string(v1alpha1.RefreshTypeNormal),
					},
					Status: v1alpha1.ApplicationStatus{
						ReconciledAt: &after,
					},
				},
			},
			expectedReconciled:   true,
			expectError:          false,
			latestTransitionTime: &before,
		},
		{
			name: "applications in waiting and not reconciled adds annotations",
			appset: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-appset",
					Namespace: "argocd",
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application:        "app1",
							Status:             v1alpha1.ProgressiveSyncWaiting,
							LastTransitionTime: &now,
							Message:            "pending changes",
						},
					},
				},
			},
			applications: []v1alpha1.Application{
				{
					Name:      "app1",
					Namespace: "argocd",
					Status: v1alpha1.ApplicationStatus{
						ReconciledAt: &before,
					},
				},
			},
			expectedReconciled:   false,
			expectError:          false,
			latestTransitionTime: &now,
		},
		{
			name: "applications without ReconciledAt adds annotations and returns false",
			appset: v1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-appset",
					Namespace: "argocd",
				},
				Status: v1alpha1.ApplicationSetStatus{
					ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
						{
							Application:        "app1",
							Status:             v1alpha1.ProgressiveSyncWaiting,
							LastTransitionTime: &now,
							Message:            "pending changes",
						},
					},
				},
			},
			applications: []v1alpha1.Application{
				{
					Name:      "app1",
					Namespace: "argocd",
					Status: v1alpha1.ApplicationStatus{
						ReconciledAt: nil,
					},
				},
			},
			expectedReconciled:   false,
			expectError:          false,
			latestTransitionTime: &now,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initObjs := []client.Object{&tt.appset}
			appObjs := []runtime.Object{}
			for i := range tt.applications {
				initObjs = append(initObjs, &tt.applications[i])
				appObjs = append(appObjs, &tt.applications[i])
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).Build()
			appclientSet := appfake.NewSimpleClientset(appObjs...)
			manager := &Manager{
				Client:       client,
				AppClientset: appclientSet,
			}

			reconciled, err := manager.ensureApplicationsReconciled(log.NewEntry(log.StandardLogger()), &tt.appset, tt.applications, tt.latestTransitionTime, 0)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedReconciled, reconciled)
			}
		})
	}
}

func TestAddRefreshAnnotationToApplications(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	tests := []struct {
		name                   string
		applications           []v1alpha1.Application
		expectError            bool
		expectAnnotationLength int
	}{
		{
			name: "adds annotation to application without annotations",
			applications: []v1alpha1.Application{
				{
					Name:      "app1",
					Namespace: "argocd",
				},
			},
			expectError:            false,
			expectAnnotationLength: 2, // call to RefreshApp adds annotations refresh and refresh-timestamp
		},
		{
			name: "adds annotation to applications with other existing annotations",
			applications: []v1alpha1.Application{
				{
					Name:      "app1",
					Namespace: "argocd",
					Annotations: map[string]string{
						"other-annotation": "value",
					},
				},
			},
			expectError:            false,
			expectAnnotationLength: 3,
		},
		{
			name: "skips application that already has refresh annotation",
			applications: []v1alpha1.Application{
				{
					Name:      "app1",
					Namespace: "argocd",
					Annotations: map[string]string{
						v1alpha1.AnnotationKeyRefresh: string(v1alpha1.RefreshTypeNormal),
					},
				},
			},
			expectError:            false,
			expectAnnotationLength: 1, // when adding annotation is skipped, call to RefreshApp is skipped, thus refresh-timestamp annotation is not added
		},
		{
			name: "adds annotation to multiple applications",
			applications: []v1alpha1.Application{
				{
					Name:      "app1",
					Namespace: "argocd",
				},
				{
					Name:      "app2",
					Namespace: "argocd",
				},
			},
			expectError:            false,
			expectAnnotationLength: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appset := v1alpha1.ApplicationSet{
				Name:      "test-appset",
				Namespace: "argocd",
			}

			initObjs := []runtime.Object{&appset}
			for i := range tt.applications {
				initObjs = append(initObjs, &tt.applications[i])
			}

			appClientSet := appfake.NewSimpleClientset(initObjs...)
			manager := &Manager{
				AppClientset: appClientSet,
			}

			err := manager.addRefreshAnnotationToApplications(log.NewEntry(log.StandardLogger()), tt.applications)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify annotations were added (except for apps that already had them)
				for _, app := range tt.applications {
					retrievedApp, err := appClientSet.ArgoprojV1alpha1().Applications(app.Namespace).Get(t.Context(), app.Name, metav1.GetOptions{})
					require.NoError(t, err)

					// Should have the refresh annotation
					assert.NotNil(t, retrievedApp.Annotations)
					assert.Equal(t, string(v1alpha1.RefreshTypeNormal), retrievedApp.Annotations[v1alpha1.AnnotationKeyRefresh])
					assert.Len(t, retrievedApp.Annotations, tt.expectAnnotationLength)
				}
			}
		})
	}
}

// TestPerformReverseDeletionStaleCache reproduces the "ApplicationSet stuck in Deleting" bug.
//
// When an ApplicationSet with deletionOrder: Reverse is deleted, the controller reads the child
// Applications from its (cache-backed) client to decide when they are gone. If the informer cache
// is stale — it still lists Applications that were already removed from the API server — then the
// per-app Get returns the ghost object while the subsequent Delete hits the API and returns
// NotFound. The old code treated that NotFound as a fatal error and returned it, so the reconcile
// error-looped forever and the ResourcesFinalizer was never removed. The only known recovery was
// restarting the controller (which rebuilds the cache from a fresh LIST).
//
// This test simulates the stale cache with an interceptor: Get succeeds (object present in the
// fake store) but Delete returns NotFound. PerformReverseDeletion must treat that as
// "already deleted" and converge to (0, nil) so the finalizer can be removed.
func TestPerformReverseDeletionStaleCache(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	appSet := v1alpha1.ApplicationSet{
		Name: "appset", Namespace: "argocd",
		Spec: v1alpha1.ApplicationSetSpec{
			Strategy: &v1alpha1.ApplicationSetStrategy{
				Type:          "RollingSync",
				DeletionOrder: ReverseDeletionOrder,
				RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
					Steps: []v1alpha1.ApplicationSetRolloutStep{
						{MatchExpressions: []v1alpha1.ApplicationMatchExpression{{Key: "stage", Operator: "In", Values: []string{"0"}}}},
						{MatchExpressions: []v1alpha1.ApplicationMatchExpression{{Key: "stage", Operator: "In", Values: []string{"1"}}}},
					},
				},
			},
		},
	}

	newApp := func(name, stage string) v1alpha1.Application {
		return v1alpha1.Application{
			Name: name, Namespace: "argocd", Labels: map[string]string{"stage": stage},
		}
	}
	// currentApps mirrors what getCurrentApplications() returns from the stale cache: both apps
	// still appear to exist even though they have already been deleted from the API server.
	app0 := newApp("appset-stage0", "0")
	app1 := newApp("appset-stage1", "1")
	currentApps := []v1alpha1.Application{app0, app1}

	deleteAttempts := map[string]int{}
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(&app0, &app1). // Get returns the ghost objects (stale cache)
		WithInterceptorFuncs(interceptor.Funcs{
			// The objects are already gone on the API server: every Delete 404s.
			Delete: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.DeleteOption) error {
				deleteAttempts[obj.GetName()]++
				return apierrors.NewNotFound(schema.GroupResource{Group: "argoproj.io", Resource: "applications"}, obj.GetName())
			},
		}).
		Build()

	m := NewManager(fakeClient, fakeClient, nil, nil)
	logCtx := log.NewEntry(log.New())

	requeue, err := m.PerformReverseDeletion(context.Background(), logCtx, appSet, currentApps)

	// With the fix, an already-deleted Application is treated as success: reverse deletion
	// converges in a single pass so the caller can remove the finalizer instead of looping.
	require.NoError(t, err, "already-deleted Application must not surface as a hard error")
	assert.Zero(t, requeue, "deletion should be complete, not requeued")
	// Both steps should have been visited and their (already-gone) deletion attempted.
	assert.Equal(t, 1, deleteAttempts["appset-stage0"])
	assert.Equal(t, 1, deleteAttempts["appset-stage1"])
}

// TestPerformReverseDeletionTerminatingApp covers an Application whose deletion is still pending.
// Whichever side of the two minute threshold it falls on, no second Delete may be issued: a
// redundant Delete on a terminating object is a no-op success, and cacheSyncingClient.Delete evicts
// the object from the informer store on success. The next Get would then 404 on a still-existing
// Application and the finalizer would be released early.
func TestPerformReverseDeletionTerminatingApp(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	appSet := v1alpha1.ApplicationSet{
		Name: "appset", Namespace: "argocd",
		Spec: v1alpha1.ApplicationSetSpec{
			Strategy: &v1alpha1.ApplicationSetStrategy{
				Type:          "RollingSync",
				DeletionOrder: ReverseDeletionOrder,
				RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
					Steps: []v1alpha1.ApplicationSetRolloutStep{
						{MatchExpressions: []v1alpha1.ApplicationMatchExpression{{Key: "stage", Operator: "In", Values: []string{"0"}}}},
					},
				},
			},
		},
	}

	for _, tc := range []struct {
		name          string
		deletionAge   time.Duration
		expectedErr   string
		expectRequeue time.Duration
	}{
		{
			name:          "within threshold",
			deletionAge:   30 * time.Second,
			expectRequeue: 10 * time.Second,
		},
		{
			// Past the threshold the reconcile fails, so the ApplicationSet finalizer stays put and
			// controller-runtime retries with backoff.
			name:        "past threshold",
			deletionAge: 5 * time.Minute,
			expectedErr: "application has not been deleted in over 2 minutes",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deletionTimestamp := metav1.NewTime(time.Now().Add(-tc.deletionAge))
			app := v1alpha1.Application{
				Name:              "appset-stage0",
				Namespace:         "argocd",
				Labels:            map[string]string{"stage": "0"},
				DeletionTimestamp: &deletionTimestamp,
				Finalizers:        []string{v1alpha1.ResourcesFinalizerName},
			}

			deleteAttempts := 0
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(&app).
				WithInterceptorFuncs(interceptor.Funcs{
					Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
						deleteAttempts++
						return c.Delete(ctx, obj, opts...)
					},
				}).
				Build()

			m := NewManager(fakeClient, fakeClient, nil, nil)
			requeue, err := m.PerformReverseDeletion(context.Background(), log.NewEntry(log.New()), appSet, []v1alpha1.Application{app})

			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.expectRequeue, requeue)
			assert.Zero(t, deleteAttempts, "must not re-Delete an already terminating Application")
		})
	}
}

func TestDelayedStatusUpdateComparedToRegression(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	appSet := v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-appset",
			Namespace: "argocd",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Strategy: &v1alpha1.ApplicationSetStrategy{
				Type: "RollingSync",
				RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
					Steps: []v1alpha1.ApplicationSetRolloutStep{
						{
							MatchExpressions: []v1alpha1.ApplicationMatchExpression{
								{Key: "env", Operator: "In", Values: []string{"dev"}},
							},
						},
					},
				},
			},
		},
		Status: v1alpha1.ApplicationSetStatus{
			ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app-dev",
					Status:          v1alpha1.ProgressiveSyncHealthy,
					Step:            "1",
					TargetRevisions: []string{"v1.0.0"},
				},
			},
		},
	}

	liveApp := v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-dev",
			Namespace: "argocd",
			Labels:    map[string]string{"env": "dev"},
		},
		Spec: v1alpha1.ApplicationSpec{
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				Path:           "guestbook",
				TargetRevision: "v1.0.0",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "default",
			},
		},
		Status: v1alpha1.ApplicationStatus{
			Health: v1alpha1.AppHealthStatus{Status: health.HealthStatusHealthy},
			Sync: v1alpha1.SyncStatus{
				Status:   v1alpha1.SyncStatusCodeSynced,
				Revision: "v1.0.0",
				ComparedTo: v1alpha1.ComparedTo{
					Source: v1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						Path:           "guestbook",
						TargetRevision: "v1.0.0",
					},
					Destination: v1alpha1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "default",
					},
				},
			},
		},
	}

	desiredApp := *liveApp.DeepCopy()
	desiredApp.Spec.Source.TargetRevision = "v2.0.0"

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet).WithStatusSubresource(&appSet).Build()
	m := NewManager(c, c, regressionDeps{})

	statuses, err := m.UpdateApplicationSetApplicationStatus(
		t.Context(),
		log.NewEntry(log.New()),
		&appSet,
		[]v1alpha1.Application{liveApp},
		[]v1alpha1.Application{desiredApp},
		map[string]int{"app-dev": 0},
	)

	require.NoError(t, err)
	require.Len(t, statuses, 1)

	// With Option A fix: stale status prevents premature progression to Healthy
	assert.Equal(t, v1alpha1.ProgressiveSyncWaiting, statuses[0].Status,
		"stale ComparedTo must not be trusted as Healthy for new desired revision")
}

func TestProgressiveSyncQodoFinding1_IgnoreDifferences(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	// AppSet declares IgnoreApplicationDifferences for targetRevision.
	// The generated desired app has a different targetRevision from the live app.
	// SpecsEquivalent sees them as equivalent (because the difference is ignored), so
	// the write path will never correct targetRevision. The freshness check must use the
	// same ignore rule via diffConfig so it does not report a permanent stale status.
	appSet := v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-appset",
			Namespace: "argocd",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			IgnoreApplicationDifferences: v1alpha1.ApplicationSetIgnoreDifferences{
				{JSONPointers: []string{"/spec/source/targetRevision"}},
			},
			Strategy: &v1alpha1.ApplicationSetStrategy{
				Type: "RollingSync",
				RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
					Steps: []v1alpha1.ApplicationSetRolloutStep{
						{
							MatchExpressions: []v1alpha1.ApplicationMatchExpression{
								{Key: "env", Operator: "In", Values: []string{"dev"}},
							},
						},
					},
				},
			},
		},
		Status: v1alpha1.ApplicationSetStatus{
			ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app-dev",
					Status:          v1alpha1.ProgressiveSyncWaiting,
					Step:            "1",
					TargetRevisions: []string{"v1.0.0"},
				},
			},
		},
	}

	liveApp := v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-dev",
			Namespace: "argocd",
			Labels:    map[string]string{"env": "dev"},
		},
		Spec: v1alpha1.ApplicationSpec{
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				Path:           "guestbook",
				TargetRevision: "v1.0.0",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "default",
			},
		},
		Status: v1alpha1.ApplicationStatus{
			Health: v1alpha1.AppHealthStatus{Status: health.HealthStatusHealthy},
			Sync: v1alpha1.SyncStatus{
				Status:   v1alpha1.SyncStatusCodeSynced,
				Revision: "v1.0.0",
				ComparedTo: v1alpha1.ComparedTo{
					Source: v1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						Path:           "guestbook",
						TargetRevision: "v1.0.0",
					},
					Destination: v1alpha1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "default",
					},
				},
			},
		},
	}

	// Desired has a different targetRevision, but the AppSet ignore rule covers that field.
	desiredApp := *liveApp.DeepCopy()
	desiredApp.Spec.Source.TargetRevision = "v2.0.0"

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet).WithStatusSubresource(&appSet).Build()
	m := NewManager(c, c, regressionDeps{})

	statuses, err := m.UpdateApplicationSetApplicationStatus(
		t.Context(),
		log.NewEntry(log.New()),
		&appSet,
		[]v1alpha1.Application{liveApp},
		[]v1alpha1.Application{desiredApp},
		map[string]int{"app-dev": 0},
	)

	require.NoError(t, err)
	require.Len(t, statuses, 1)

	// IgnoreApplicationDifferences ignores the targetRevision field on the AppSet level.
	// SpecsEquivalent sees the live and desired apps as equivalent. The freshness check must
	// then use the live app's normalized spec (targetRevision=v1.0.0) against ComparedTo
	// (which also records v1.0.0), so statusIsFresh=true and the app advances to Healthy.
	// Without the fix, the raw desired spec (v2.0.0) would not match ComparedTo (v1.0.0),
	// leaving the app permanently stuck in Waiting.
	assert.Equal(t, v1alpha1.ProgressiveSyncHealthy, statuses[0].Status,
		"a field covered by IgnoreApplicationDifferences must not cause a false stale-status: %s", statuses[0].Message)
}

func TestProgressiveSyncQodoFinding2_Normalization(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	// The generated desired app has an empty-but-present Kustomize struct (kustomize: {}).
	// NormalizeApplicationSpec collapses it to nil before writing to ComparedTo.
	// The freshness check must normalize before building expectedComparedTo so it matches the
	// Application controller's recorded nil ComparedTo.Source.Kustomize.
	appSet := v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-appset",
			Namespace: "argocd",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Strategy: &v1alpha1.ApplicationSetStrategy{
				Type: "RollingSync",
				RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
					Steps: []v1alpha1.ApplicationSetRolloutStep{
						{
							MatchExpressions: []v1alpha1.ApplicationMatchExpression{
								{Key: "env", Operator: "In", Values: []string{"dev"}},
							},
						},
					},
				},
			},
		},
		Status: v1alpha1.ApplicationSetStatus{
			ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{
					Application:     "app-dev",
					Status:          v1alpha1.ProgressiveSyncWaiting,
					Step:            "1",
					TargetRevisions: []string{"v1.0.0"},
				},
			},
		},
	}

	// Live app has nil Kustomize (as the Application controller stored it).
	liveApp := v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-dev",
			Namespace: "argocd",
			Labels:    map[string]string{"env": "dev"},
		},
		Spec: v1alpha1.ApplicationSpec{
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				Path:           "guestbook",
				TargetRevision: "v1.0.0",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "default",
			},
		},
		Status: v1alpha1.ApplicationStatus{
			Health: v1alpha1.AppHealthStatus{Status: health.HealthStatusHealthy},
			Sync: v1alpha1.SyncStatus{
				Status:   v1alpha1.SyncStatusCodeSynced,
				Revision: "v1.0.0",
				ComparedTo: v1alpha1.ComparedTo{
					Source: v1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						Path:           "guestbook",
						TargetRevision: "v1.0.0",
					},
					Destination: v1alpha1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "default",
					},
				},
			},
		},
	}

	desiredApp := *liveApp.DeepCopy()
	// Generated desired app has an unnormalized empty Kustomize struct.
	desiredApp.Spec.Source.Kustomize = &v1alpha1.ApplicationSourceKustomize{}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appSet).WithStatusSubresource(&appSet).Build()
	m := NewManager(c, c, regressionDeps{})

	statuses, err := m.UpdateApplicationSetApplicationStatus(
		t.Context(),
		log.NewEntry(log.New()),
		&appSet,
		[]v1alpha1.Application{liveApp},
		[]v1alpha1.Application{desiredApp},
		map[string]int{"app-dev": 0},
	)

	require.NoError(t, err)
	require.Len(t, statuses, 1)

	// After NormalizeApplicationSpec, the empty Kustomize struct becomes nil, making the desired
	// spec identical to the live spec. The freshness check normalizes before comparing to
	// ComparedTo, so statusIsFresh=true and the app advances to Healthy instead of staying Waiting.
	assert.Equal(t, v1alpha1.ProgressiveSyncHealthy, statuses[0].Status,
		"unnormalized empty struct (kustomize: {}) must not cause a permanent ComparedTo mismatch: %s", statuses[0].Message)
}
