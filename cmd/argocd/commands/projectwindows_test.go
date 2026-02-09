package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestPrintSyncWindows(t *testing.T) {
	proj := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "test-project"},
		Spec: v1alpha1.AppProjectSpec{
			SyncWindows: v1alpha1.SyncWindows{
				{
					Kind:           "allow",
					Schedule:       "* * * * *",
					Duration:       "1h",
					Applications:   []string{"app1"},
					Namespaces:     []string{"ns1"},
					Clusters:       []string{"cluster1"},
					ManualSync:     true,
					UseAndOperator: true,
				},
			},
		},
	}

	output, err := captureOutput(func() error {
		printSyncWindows(proj)
		return nil
	})
	require.NoError(t, err)
	t.Log(output)
	assert.Contains(t, output, "ID  STATUS  KIND   SCHEDULE   DURATION  APPLICATIONS  NAMESPACES  CLUSTERS  MANUALSYNC  TIMEZONE  USEANDOPERATOR")
	assert.Contains(t, output, "0   Active  allow  * * * * *  1h        app1          ns1         cluster1  Enabled               Enabled")
}
