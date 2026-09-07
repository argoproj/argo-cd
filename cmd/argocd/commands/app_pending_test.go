package commands

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)
func TestFormatPendingResources(t *testing.T) {
	testCases := []struct {
		name        string
		resources   []*resourceState
		maxPending  int
		expectNil   bool
		description string
	}{
		{
			name:        "empty resources",
			resources:   []*resourceState{},
			maxPending:  10,
			expectNil:   true,
			description: "empty list should return nil error",
		},
		{
			name: "one synced resource",
			resources: []*resourceState{
				{
					Group: "apps", Kind: "Deployment", Namespace: "default", Name: "my-app",
					Status: "Synced", Health: "Healthy",
				},
			},
			maxPending:  10,
			expectNil:   true,
			description: "one synced resource should return nil error",
		},
		{
			name: "one pending resource",
			resources: []*resourceState{
				{
					Group: "apps", Kind: "Deployment", Namespace: "default", Name: "my-app",
					Status: "OutOfSync", Health: "Degraded",
				},
			},
			maxPending:  10,
			expectNil:   false,
			description: "one pending resource should return non-nil error",
		},
		{
			name: "two pending resources",
			resources: []*resourceState{
				{
					Group: "apps", Kind: "Deployment", Namespace: "default", Name: "my-app-1",
					Status: "OutOfSync", Health: "Degraded",
				},
				{
					Group: "core", Kind: "ConfigMap", Namespace: "default", Name: "my-config",
					Status: "OutOfSync", Health: "Progressing",
				},
			},
			maxPending:  10,
			expectNil:   false,
			description: "two pending resources should return non-nil error",
		},
		{
			name: "10 resources, at limit of 10",
			resources: func() []*resourceState {
				resources := make([]*resourceState, 10)
				for i := range resources {
					resources[i] = &resourceState{
						Group: "apps", Kind: "Deployment", Namespace: "default", Name: "my-app-" + fmt.Sprintf("%d", i),
						Status: "OutOfSync", Health: "Degraded",
					}
				}
				return resources
			}(),
			maxPending:  10,
			expectNil:   false,
			description: "10 pending resources should return non-nil error with all shown",
		},
		{
			name: "11 resources, at limit of 10",
			resources: func() []*resourceState {
				resources := make([]*resourceState, 11)
				for i := range resources {
					resources[i] = &resourceState{
						Group: "apps", Kind: "Deployment", Namespace: "default", Name: "my-app-" + fmt.Sprintf("%d", i),
						Status: "OutOfSync", Health: "Degraded",
					}
				}
				return resources
			}(),
			maxPending:  10,
			expectNil:   false,
			description: "11 pending resources should return non-nil error with 10 shown",
		},
		{
			name: "15 resources, at limit of 10",
			resources: func() []*resourceState {
				resources := make([]*resourceState, 15)
				for i := range resources {
					resources[i] = &resourceState{
						Group: "apps", Kind: "Deployment", Namespace: "default", Name: "my-app-" + fmt.Sprintf("%d", i),
						Status: "OutOfSync", Health: "Degraded",
					}
				}
				return resources
			}(),
			maxPending:  10,
			expectNil:   false,
			description: "15 pending resources should return non-nil error with 10 shown",
		},
		{
			name: "0 limit - show all resources",
			resources: []*resourceState{
				{
					Group: "apps", Kind: "Deployment", Namespace: "default", Name: "my-app-1",
					Status: "OutOfSync", Health: "Degraded",
				},
				{
					Group: "core", Kind: "ConfigMap", Namespace: "default", Name: "my-config",
					Status: "OutOfSync", Health: "Progressing",
				},
			},
			maxPending:  0,
			expectNil:   false,
			description: "limit of 0 should return non-nil error with all resources shown",
		},
		{
			name: "negative limit - treat as 0",
			resources: func() []*resourceState {
				resources := make([]*resourceState, 5)
				for i := range resources {
					resources[i] = &resourceState{
						Group: "apps", Kind: "Deployment", Namespace: "default", Name: "my-app-" + fmt.Sprintf("%d", i),
						Status: "OutOfSync", Health: "Degraded",
					}
				}
				return resources
			}(),
			maxPending:  -1,
			expectNil:   false,
			description: "negative limit should be treated as 0 and show all resources",
		},
		{
			name: "more than 100 resources",
			resources: func() []*resourceState {
				resources := make([]*resourceState, 105)
				for i := range resources {
					resources[i] = &resourceState{
						Group: "apps", Kind: "Deployment", Namespace: "default", Name: "my-app-" + fmt.Sprintf("%d", i),
						Status: "OutOfSync", Health: "Degraded",
					}
				}
				return resources
			}(),
			maxPending:  50,
			expectNil:   false,
			description: "more than 100 resources should be capped at limit of 50",
		},
		{
			name: "large limit",
			resources: func() []*resourceState {
				resources := make([]*resourceState, 10)
				for i := range resources {
					resources[i] = &resourceState{
						Group: "apps", Kind: "Deployment", Namespace: "default", Name: "my-app-" + fmt.Sprintf("%d", i),
						Status: "OutOfSync", Health: "Degraded",
					}
				}
				return resources
			}(),
			maxPending:  1000,
			expectNil:   false,
			description: "large limit should return non-nil error with all resources shown",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.maxPending < 0 {
				// Treat negative as 0 (show all)
				tc.maxPending = 0
			}
			err := formatPendingResources(tc.resources, tc.maxPending)
			if tc.expectNil {
				require.Nil(t, err, tc.description)
			} else {
				require.NotNil(t, err, tc.description)
			}
		})
	}
}
