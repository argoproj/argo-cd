package progressivesync

import (
	"context"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

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
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-dev",
						Labels: map[string]string{
							"env": "dev",
						},
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa",
						Labels: map[string]string{
							"env": "qa",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-prod",
						Labels: map[string]string{
							"env": "prod",
						},
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa1",
						Labels: map[string]string{
							"env": "qa",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa2",
						Labels: map[string]string{
							"env":    "qa",
							"region": "us-east-2",
						},
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-dev",
						Labels: map[string]string{
							"env": "dev",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa",
						Labels: map[string]string{
							"env": "qa",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-prod",
						Labels: map[string]string{
							"env":    "prod",
							"region": "us-east-2",
						},
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-dev",
						Labels: map[string]string{
							"env": "dev",
						},
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa",
						Labels: map[string]string{
							"env": "qa",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-prod",
						Labels: map[string]string{
							"env": "prod",
						},
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa1",
						Labels: map[string]string{
							"env": "qa",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa2",
						Labels: map[string]string{
							"env":    "qa",
							"region": "us-east-2",
						},
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa1",
						Labels: map[string]string{
							"env":    "qa",
							"region": "us-east-1",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa2",
						Labels: map[string]string{
							"env":    "qa",
							"region": "us-east-2",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-prod1",
						Labels: map[string]string{
							"env":    "prod",
							"region": "us-east-1",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-prod2",
						Labels: map[string]string{
							"env":    "prod",
							"region": "us-east-2",
						},
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-dev",
						Labels: map[string]string{
							"env": "dev",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa",
						Labels: map[string]string{
							"env": "qa",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-prod",
						Labels: map[string]string{
							"env":    "prod",
							"region": "us-east-2",
						},
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-dev",
						Labels: map[string]string{
							"env": "dev",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa1",
						Labels: map[string]string{
							"env":    "qa",
							"region": "us-west-2",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa2",
						Labels: map[string]string{
							"env":    "qa",
							"region": "us-east-2",
						},
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-dev",
						Labels: map[string]string{
							"env":    "dev",
							"region": "us-west-2",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-qa",
						Labels: map[string]string{
							"region": "us-west-2",
						},
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-dev",
						Labels: map[string]string{
							"env": "dev",
						},
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

func TestGetAppsToSyncWithParallelGroups(t *testing.T) {
	appSet := v1alpha1.ApplicationSet{
		Status: v1alpha1.ApplicationSetStatus{
			ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{Application: "app1", Status: v1alpha1.ProgressiveSyncHealthy},
				{Application: "app2", Status: v1alpha1.ProgressiveSyncProgressing},
			},
		},
	}
	currentApps := []v1alpha1.Application{
		{ObjectMeta: metav1.ObjectMeta{Name: "app1"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "app2"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "app3"}},
	}

	got := getAppsToSyncWithGroups(
		appSet,
		[][]string{{"app1"}, {"app2"}, {"app3"}},
		[][]int{{0, 1}, {2}},
		currentApps,
	)

	assert.Equal(t, map[string]bool{"app1": true, "app2": true}, got)
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
				{ObjectMeta: metav1.ObjectMeta{Name: "app2"}},
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
				{ObjectMeta: metav1.ObjectMeta{Name: "app1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "app2"}},
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
				{ObjectMeta: metav1.ObjectMeta{Name: "app1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "app2"}},
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
				{ObjectMeta: metav1.ObjectMeta{Name: "app1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "app2"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "app3"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "app4"}},
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
				{ObjectMeta: metav1.ObjectMeta{Name: "app1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "app2"}},
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
				{ObjectMeta: metav1.ObjectMeta{Name: "app1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "app2"}},
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
				{ObjectMeta: metav1.ObjectMeta{Name: "app1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "app2"}},
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
				{ObjectMeta: metav1.ObjectMeta{Name: "old_app"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "app1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "app2"}},
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
		name     string
		input    v1alpha1.Application
		prune    bool
		expected v1alpha1.Application
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := syncApplication(tt.input, tt.prune)
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
		ObjectMeta: metav1.ObjectMeta{Name: "appset", Namespace: "argocd"},
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
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "argocd", Labels: map[string]string{"stage": stage}},
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

	m := NewManager(fakeClient, nil)
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
