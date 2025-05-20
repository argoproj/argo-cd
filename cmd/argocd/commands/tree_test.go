package commands

import (
	"bytes"
	"testing"
	"text/tabwriter"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestTreeViewAppGet(t *testing.T) {
	var parent v1alpha1.ResourceNode
	parent.ResourceRef = v1alpha1.ResourceRef{Group: "argoproj.io", Version: "", Kind: "Rollout", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo", UID: "87f3aab0-f634-4b2c-959a-7ddd30675ed0"}
	objs := make(map[string]v1alpha1.ResourceNode)
	objs["87f3aab0-f634-4b2c-959a-7ddd30675ed0"] = parent
	var child v1alpha1.ResourceNode
	child.ResourceRef = v1alpha1.ResourceRef{Group: "apps", Version: "v1", Kind: "ReplicaSet", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo-5dcd5457d5", UID: "75c30dce-1b66-414f-a86c-573a74be0f40"}
	child.ParentRefs = []v1alpha1.ResourceRef{{Group: "argoproj.io", Version: "", Kind: "Rollout", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo", UID: "87f3aab0-f634-4b2c-959a-7ddd30675ed0"}}

	objs["75c30dce-1b66-414f-a86c-573a74be0f40"] = child

	childMapping := make(map[string][]string)
	childMapping["87f3aab0-f634-4b2c-959a-7ddd30675ed0"] = []string{"75c30dce-1b66-414f-a86c-573a74be0f40"}

	stateMap := make(map[string]*resourceState)
	stateMap["Rollout/numalogic-rollout-demo"] = &resourceState{
		Status:  "Running",
		Health:  "Healthy",
		Hook:    "",
		Message: "No Issues",
		Name:    "sandbox-rollout-numalogic-demo",
		Kind:    "Rollout",
		Group:   "argoproj.io",
	}

	buf := &bytes.Buffer{}
	w := tabwriter.NewWriter(buf, 0, 0, 2, ' ', 0)
	treeViewAppGet("", objs, childMapping, parent, stateMap, w)
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	assert.Contains(t, output, "ReplicaSet")
	assert.Contains(t, output, "Rollout")
	assert.Contains(t, output, "numalogic-rollout")
	assert.Contains(t, output, "Healthy")
	assert.Contains(t, output, "No Issues")
}

func TestTreeViewDetailedAppGet(t *testing.T) {
	var parent v1alpha1.ResourceNode
	parent.ResourceRef = v1alpha1.ResourceRef{Group: "argoproj.io", Version: "", Kind: "Rollout", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo", UID: "87f3aab0-f634-4b2c-959a-7ddd30675ed0"}
	objs := make(map[string]v1alpha1.ResourceNode)
	objs["87f3aab0-f634-4b2c-959a-7ddd30675ed0"] = parent
	var child v1alpha1.ResourceNode
	child.ResourceRef = v1alpha1.ResourceRef{Group: "apps", Version: "v1", Kind: "ReplicaSet", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo-5dcd5457d5", UID: "75c30dce-1b66-414f-a86c-573a74be0f40"}
	child.ParentRefs = []v1alpha1.ResourceRef{{Group: "argoproj.io", Version: "", Kind: "Rollout", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo", UID: "87f3aab0-f634-4b2c-959a-7ddd30675ed0"}}
	child.Health = &v1alpha1.HealthStatus{Status: "Degraded", Message: "Readiness Gate failed"}
	objs["75c30dce-1b66-414f-a86c-573a74be0f40"] = child

	childMapping := make(map[string][]string)
	childMapping["87f3aab0-f634-4b2c-959a-7ddd30675ed0"] = []string{"75c30dce-1b66-414f-a86c-573a74be0f40"}

	stateMap := make(map[string]*resourceState)
	stateMap["Rollout/numalogic-rollout-demo"] = &resourceState{
		Status:  "Running",
		Health:  "Healthy",
		Hook:    "",
		Message: "No Issues",
		Name:    "sandbox-rollout-numalogic-demo",
		Kind:    "Rollout",
		Group:   "argoproj.io",
	}

	buf := &bytes.Buffer{}
	w := tabwriter.NewWriter(buf, 0, 0, 2, ' ', 0)
	detailedTreeViewAppGet("", objs, childMapping, parent, stateMap, w)
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	output := buf.String()

	assert.Contains(t, output, "ReplicaSet")
	assert.Contains(t, output, "Rollout")
	assert.Contains(t, output, "numalogic-rollout")
	assert.Contains(t, output, "Healthy")
	assert.Contains(t, output, "No Issues")
	assert.Contains(t, output, "Degraded")
	assert.Contains(t, output, "Readiness Gate failed")
}

func TestTreeViewAppResources(t *testing.T) {
	var parent v1alpha1.ResourceNode
	parent.ResourceRef = v1alpha1.ResourceRef{Group: "argoproj.io", Version: "", Kind: "Rollout", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo", UID: "87f3aab0-f634-4b2c-959a-7ddd30675ed0"}
	objs := make(map[string]v1alpha1.ResourceNode)
	objs["87f3aab0-f634-4b2c-959a-7ddd30675ed0"] = parent
	var child v1alpha1.ResourceNode
	child.ResourceRef = v1alpha1.ResourceRef{Group: "apps", Version: "v1", Kind: "ReplicaSet", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo-5dcd5457d5", UID: "75c30dce-1b66-414f-a86c-573a74be0f40"}
	child.ParentRefs = []v1alpha1.ResourceRef{{Group: "argoproj.io", Version: "", Kind: "Rollout", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo", UID: "87f3aab0-f634-4b2c-959a-7ddd30675ed0"}}

	objs["75c30dce-1b66-414f-a86c-573a74be0f40"] = child

	childMapping := make(map[string][]string)
	childMapping["87f3aab0-f634-4b2c-959a-7ddd30675ed0"] = []string{"75c30dce-1b66-414f-a86c-573a74be0f40"}

	buf := &bytes.Buffer{}
	w := tabwriter.NewWriter(buf, 0, 0, 2, ' ', 0)

	treeViewAppResourcesNotOrphaned("", objs, childMapping, parent, w)

	var orphan v1alpha1.ResourceNode
	orphan.ResourceRef = v1alpha1.ResourceRef{Group: "apps", Version: "v1", Kind: "ReplicaSet", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo-5dcdnk457d5", UID: "75c30dce-1b66-41hf-a86c-573a74be0f40"}
	objsOrphan := make(map[string]v1alpha1.ResourceNode)
	objsOrphan["75c30dce-1b66-41hf-a86c-573a74be0f40"] = orphan
	orphanchildMapping := make(map[string][]string)
	orphanParent := orphan

	treeViewAppResourcesOrphaned("", objsOrphan, orphanchildMapping, orphanParent, w)
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	assert.Contains(t, output, "ReplicaSet")
	assert.Contains(t, output, "Rollout")
	assert.Contains(t, output, "numalogic-rollout")
	assert.Contains(t, output, "argoproj.io")
	assert.Contains(t, output, "No")
	assert.Contains(t, output, "Yes")
	assert.Contains(t, output, "numalogic-rollout-demo-5dcdnk457d5")
}

func TestTreeViewDetailedAppResources(t *testing.T) {
	var parent v1alpha1.ResourceNode
	parent.ResourceRef = v1alpha1.ResourceRef{Group: "argoproj.io", Version: "", Kind: "Rollout", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo", UID: "87f3aab0-f634-4b2c-959a-7ddd30675ed0"}
	objs := make(map[string]v1alpha1.ResourceNode)
	objs["87f3aab0-f634-4b2c-959a-7ddd30675ed0"] = parent
	var child v1alpha1.ResourceNode
	child.ResourceRef = v1alpha1.ResourceRef{Group: "apps", Version: "v1", Kind: "ReplicaSet", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo-5dcd5457d5", UID: "75c30dce-1b66-414f-a86c-573a74be0f40"}
	child.ParentRefs = []v1alpha1.ResourceRef{{Group: "argoproj.io", Version: "", Kind: "Rollout", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo", UID: "87f3aab0-f634-4b2c-959a-7ddd30675ed0"}}
	objs["75c30dce-1b66-414f-a86c-573a74be0f40"] = child
	childMapping := make(map[string][]string)
	childMapping["87f3aab0-f634-4b2c-959a-7ddd30675ed0"] = []string{"75c30dce-1b66-414f-a86c-573a74be0f40"}
	buf := &bytes.Buffer{}
	w := tabwriter.NewWriter(buf, 0, 0, 2, ' ', 0)
	detailedTreeViewAppResourcesNotOrphaned("", objs, childMapping, parent, w)
	var orphan v1alpha1.ResourceNode
	orphan.ResourceRef = v1alpha1.ResourceRef{Group: "apps", Version: "v1", Kind: "ReplicaSet", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo-5dcdnk457d5", UID: "75c30dce-1b66-41hf-a86c-573a74be0f40"}
	orphan.Health = &v1alpha1.HealthStatus{
		Status:  "Degraded",
		Message: "Readiness Gate failed",
	}
	objsOrphan := make(map[string]v1alpha1.ResourceNode)
	objsOrphan["75c30dce-1b66-41hf-a86c-573a74be0f40"] = orphan

	orphanchildMapping := make(map[string][]string)
	orphanParent := orphan
	detailedTreeViewAppResourcesOrphaned("", objsOrphan, orphanchildMapping, orphanParent, w)
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	assert.Contains(t, output, "ReplicaSet")
	assert.Contains(t, output, "Rollout")
	assert.Contains(t, output, "numalogic-rollout")
	assert.Contains(t, output, "argoproj.io")
	assert.Contains(t, output, "No")
	assert.Contains(t, output, "Yes")
	assert.Contains(t, output, "numalogic-rollout-demo-5dcdnk457d5")
	assert.Contains(t, output, "Degraded")
	assert.Contains(t, output, "Readiness Gate failed")
}

func TestPrintPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		name     string
	}{
		{
			input:    "",
			expected: "",
			name:     "empty string",
		},
		{
			input:    firstElemPrefix,
			expected: firstElemPrefix,
			name:     "only first element prefix",
		},
		{
			input:    lastElemPrefix,
			expected: lastElemPrefix,
			name:     "only last element prefix",
		},
		{
			input:    firstElemPrefix + firstElemPrefix,
			expected: pipe + firstElemPrefix,
			name:     "double first element prefix",
		},
		{
			input:    firstElemPrefix + lastElemPrefix,
			expected: pipe + lastElemPrefix,
			name:     "first then last element prefix",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := printPrefix(test.input)
			assert.Equal(t, test.expected, got)
		})
	}
}
