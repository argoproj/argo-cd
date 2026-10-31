package controller

// This file is the test of the normalizeTargetResources contract (see its doc
// comment in sync.go): what a RespectIgnoreDifferences sync applies, given the
// git manifest, the live object, the application's ignore rules and the
// cluster's schema.
//
// Every case goes through normalizeTargetResources itself, never through its
// helpers, so the behavior is locked in at the seam the sync uses and the
// implementation underneath can change freely. Each case takes a real object,
// states the behavior a user configuring ignoreDifferences expects, asserts it
// on the apply target, and where the cluster outcome is not obvious replays
// the apply target with the real server-side-apply field manager (and, for
// built-ins, kubectl's client-side three-way merge).
//
// The cases come first, in the order they were found, each with a "Ref:"
// naming the issue, PR or guide that raised it; the harness is at the end.

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/apimachinery/pkg/util/managedfields/managedfieldstest"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kube-openapi/pkg/util/proto"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync"
	enginescheme "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube/scheme"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo/diff"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
)

const (
	argoManager  = "argocd-controller"
	otherManager = "other-controller"
)

// ---------------------------------------------------------------------------
// Use case 1: merge-keyed list (initContainers), whole entry injected and ignored.
// This is the example from docs/user-guide/diffing.md. The ignored entry exists
// only in live and must be carried into the apply target.
//
// Ref: the entry was dropped since https://github.com/argoproj/argo-cd/pull/27136
// (a list was treated as a leaf when the ignored item exists only in live);
// first reported for Ingress paths in
// https://github.com/argoproj/argo-cd/issues/29944. The init-container
// variant was raised while reviewing https://github.com/argoproj/argo-cd/pull/29646.
// ---------------------------------------------------------------------------

const injectedInitTarget = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      initContainers:
      - name: wait-for-db
        image: busybox:1.36
        args: ["sh", "-c", "until nc -z db 5432; do sleep 1; done"]
      containers:
      - name: web
        image: nginx:1.27
`

const injectedInitLive = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      initContainers:
      - name: wait-for-db
        image: busybox:1.36
        args: ["sh", "-c", "until nc -z db 5432; do sleep 1; done"]
      - name: injected-init-container
        image: registry.example.com/injector:1.4.2
        args: ["--mode=init"]
      containers:
      - name: web
        image: nginx:1.27
`

// Ref: https://github.com/argoproj/argo-cd/issues/29944 (same mechanism), docs/user-guide/diffing.md example.
func TestRespectIgnoreSemantics_InjectedInitContainerSurvivesSync_GitHasItsOwnInitContainer(t *testing.T) {
	live := toUnstructured(t, injectedInitLive)
	target := toUnstructured(t, injectedInitTarget)
	ignore := deploymentIgnore(`.spec.template.spec.initContainers[] | select(.name == "injected-init-container")`)

	applied := applyTarget(t, nil, live, target, ignore)

	// The ignored entry is live state Argo CD must not touch, so the apply
	// target has to carry it: initContainers is a list-map keyed by name, and
	// an apply that omits an entry Argo CD previously applied deletes it.
	assert.ElementsMatch(t, []string{"wait-for-db", "injected-init-container"}, names(podList(t, applied, "initContainers")),
		"ignored injected init container was dropped from the apply target")

	// Consequence under SSA: every earlier sync applied the injected
	// container (RespectIgnoreDifferences copied it), so argocd-controller
	// owns it. An apply target without it removes it from the live Deployment.
	t.Run("ssa", func(t *testing.T) {
		sim := newSSASim(t, builtinTypeConverter(t, deploymentGVK), deploymentGVK)
		sim.apply(live, argoManager) // what earlier syncs applied
		sim.apply(applied, argoManager)
		assert.ElementsMatch(t, []string{"wait-for-db", "injected-init-container"}, names(podList(t, sim.live(), "initContainers")),
			"SSA removed the injected init container from the live Deployment")
	})
	// Consequence under CSA: last-applied (from the earlier sync) contains
	// the entry; if the new config does not, the three-way patch deletes it.
	t.Run("csa", func(t *testing.T) {
		out := csaApply(t, live, live, applied)
		assert.ElementsMatch(t, []string{"wait-for-db", "injected-init-container"}, names(podList(t, out, "initContainers")),
			"CSA removed the injected init container from the live Deployment")
	})
}

// Ref: https://github.com/argoproj/argo-cd/pull/29646 (the `len == 0` special case added there is what this covers).
func TestRespectIgnoreSemantics_InjectedInitContainerSurvivesSync_GitHasNoInitContainers(t *testing.T) {
	live := toUnstructured(t, injectedInitLive)
	target := toUnstructured(t, injectedInitTarget)
	// Same as above, but the manifest declares no init containers at all, so
	// the ignored entry is the whole live list. (#29646 special-cased an
	// emptied live-only list for exactly this shape.)
	unstructured.RemoveNestedField(target.Object, "spec", "template", "spec", "initContainers")
	liveInits := podList(t, live, "initContainers")
	require.NoError(t, unstructured.SetNestedSlice(live.Object, liveInits[1:], "spec", "template", "spec", "initContainers"))
	ignore := deploymentIgnore(`.spec.template.spec.initContainers[] | select(.name == "injected-init-container")`)

	applied := applyTarget(t, nil, live, target, ignore)

	assert.ElementsMatch(t, []string{"injected-init-container"}, names(podList(t, applied, "initContainers")),
		"ignored injected init container was dropped from the apply target")

	t.Run("ssa", func(t *testing.T) {
		sim := newSSASim(t, builtinTypeConverter(t, deploymentGVK), deploymentGVK)
		sim.apply(live, argoManager)
		sim.apply(applied, argoManager)
		assert.ElementsMatch(t, []string{"injected-init-container"}, names(podList(t, sim.live(), "initContainers")))
	})
}

// Ref: raised while reviewing https://github.com/argoproj/argo-cd/pull/29646 ("lists still lost" for a
// partially ignored live-only entry); no public report of this exact shape yet.
func TestRespectIgnoreSemantics_InjectedInitContainerImageIgnored_EntryKept(t *testing.T) {
	live := toUnstructured(t, injectedInitLive)
	target := toUnstructured(t, injectedInitTarget)
	unstructured.RemoveNestedField(target.Object, "spec", "template", "spec", "initContainers")
	liveInits := podList(t, live, "initContainers")
	require.NoError(t, unstructured.SetNestedSlice(live.Object, liveInits[1:], "spec", "template", "spec", "initContainers"))
	// The injector bumps its own image; the user ignores only that field.
	ignore := deploymentIgnore(`.spec.template.spec.initContainers[] | select(.name == "injected-init-container") | .image`)

	applied := applyTarget(t, nil, live, target, ignore)

	// Only the image is ignored, so the apply target should assert exactly
	// the list-map entry {name, image: <live>}, which is what earlier
	// controllers applied. Asserting the other fields of a container the
	// manifest does not declare would be wrong; omitting the entry drops a
	// field Argo CD owns.
	inits := podList(t, applied, "initContainers")
	require.Len(t, inits, 1, "injected init container entry was dropped from the apply target")
	assert.Equal(t, map[string]any{"name": "injected-init-container", "image": "registry.example.com/injector:1.4.2"}, inits[0])

	t.Run("ssa", func(t *testing.T) {
		sim := newSSASim(t, builtinTypeConverter(t, deploymentGVK), deploymentGVK)
		// an earlier sync applied {name, image} for the injected container
		preUpgrade := target.DeepCopy()
		require.NoError(t, unstructured.SetNestedSlice(preUpgrade.Object, []any{map[string]any{"name": "injected-init-container", "image": "registry.example.com/injector:1.4.2"}}, "spec", "template", "spec", "initContainers"))
		sim.apply(preUpgrade, argoManager)
		sim.update(live, otherManager) // injector fills in the rest
		sim.apply(applied, argoManager)
		assert.ElementsMatch(t, []string{"injected-init-container"}, names(podList(t, sim.live(), "initContainers")),
			"SSA removed the injected init container from the live Deployment")
	})
}

// ---------------------------------------------------------------------------
// Use case 2: merge-keyed list entry present in both, ignored field absent from
// the manifest. The VPA recipe: resources are managed in-cluster and the
// manifest does not set them.
//
// Ref: raised while reviewing https://github.com/argoproj/argo-cd/pull/29646
// (an ignored field inside a keyed-list entry); dropped since
// https://github.com/argoproj/argo-cd/pull/27136. No public report yet.
// ---------------------------------------------------------------------------

const vpaTarget = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: api
  template:
    metadata:
      labels:
        app: api
    spec:
      containers:
      - name: api
        image: registry.example.com/api:2.3.0
        ports:
        - containerPort: 8080
`

const vpaLive = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: api
  template:
    metadata:
      labels:
        app: api
    spec:
      containers:
      - name: api
        image: registry.example.com/api:2.3.0
        ports:
        - containerPort: 8080
        resources:
          requests:
            cpu: 250m
            memory: 512Mi
          limits:
            memory: 1Gi
`

// Ref: https://github.com/argoproj/argo-cd/pull/29646 review, https://github.com/argoproj/argo-cd/pull/27136.
func TestRespectIgnoreSemantics_VPAManagedResourcesKept_GitDeclaresNone(t *testing.T) {
	live := toUnstructured(t, vpaLive)
	target := toUnstructured(t, vpaTarget)
	ignore := deploymentIgnore(`.spec.template.spec.containers[].resources`)

	applied := applyTarget(t, nil, live, target, ignore)

	liveResources := at(t, live, "spec.template.spec.containers[0].resources")
	assert.Equal(t, liveResources, at(t, applied, "spec.template.spec.containers[0].resources"),
		"the VPA's resources stay on the container")

	// CSA: an earlier sync wrote the resources into last-applied. If the new
	// config omits them, kubectl's three-way patch deletes them, and the VPA
	// re-adds them only on its next recommendation cycle (in "Initial" mode
	// never for running pods).
	t.Run("csa", func(t *testing.T) {
		out := csaApply(t, live, live, applied)
		assert.Equal(t, liveResources, at(t, out, "spec.template.spec.containers[0].resources"),
			"CSA removed the VPA-managed resources from the live Deployment")
	})
	// SSA: the VPA updated the fields (so it co-owns them); argocd-controller
	// omitting them only drops its own ownership. Recorded for completeness.
	t.Run("ssa", func(t *testing.T) {
		sim := newSSASim(t, builtinTypeConverter(t, deploymentGVK), deploymentGVK)
		sim.apply(target, argoManager)
		sim.update(live, otherManager) // VPA sets resources
		sim.apply(live, argoManager)   // an earlier sync re-applied them
		sim.apply(applied, argoManager)
		assert.Equal(t, liveResources, at(t, sim.live(), "spec.template.spec.containers[0].resources"))
	})
}

// ---------------------------------------------------------------------------
// Use case 3: atomic list (tolerations). No merge key: the API server replaces
// the whole list on every apply, so the apply target must carry the ignored
// live item alongside the git items.
//
// Ref: live-only ignored item dropped: https://github.com/argoproj/argo-cd/issues/29944.
// Non-ignored sibling clobbered by the live list:
// https://github.com/argoproj/argo-cd/issues/17694 (original "ignore individual
// array elements" issue), https://github.com/argoproj/argo-cd/issues/23283.
// ---------------------------------------------------------------------------

const tolerationsTarget = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: worker
  template:
    metadata:
      labels:
        app: worker
    spec:
      tolerations:
      - key: dedicated
        operator: Equal
        value: workers
        effect: NoSchedule
      containers:
      - name: worker
        image: registry.example.com/worker:5.1.0
`

const tolerationsLive = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: worker
  template:
    metadata:
      labels:
        app: worker
    spec:
      tolerations:
      - key: dedicated
        operator: Equal
        value: workers
        effect: NoSchedule
      - key: node.example.com/spot
        operator: Exists
        effect: NoSchedule
      containers:
      - name: worker
        image: registry.example.com/worker:5.1.0
`

// Ref: https://github.com/argoproj/argo-cd/issues/29944.
func TestRespectIgnoreSemantics_InjectedTolerationSurvivesSync(t *testing.T) {
	live := toUnstructured(t, tolerationsLive)
	target := toUnstructured(t, tolerationsTarget)
	// A policy engine (e.g. a Kyverno mutate rule on Deployments) appends a
	// spot toleration. The user ignores that entry.
	ignore := deploymentIgnore(`.spec.template.spec.tolerations[] | select(.key == "node.example.com/spot")`)

	applied := applyTarget(t, nil, live, target, ignore)

	// tolerations is atomic: the apply target either asserts the whole list
	// or nothing. To leave the ignored entry untouched it must include it.
	assert.ElementsMatch(t, []string{"dedicated", "node.example.com/spot"}, names(podList(t, applied, "tolerations")),
		"ignored toleration was dropped from the apply target")

	// SSA: whoever applies an atomic list owns the whole list. Argo CD's apply
	// replaces it with [dedicated] and the injected toleration is gone, even
	// when another manager set it.
	t.Run("ssa", func(t *testing.T) {
		sim := newSSASim(t, builtinTypeConverter(t, deploymentGVK), deploymentGVK)
		sim.apply(target, argoManager)
		sim.update(live, otherManager) // policy engine appends the toleration
		sim.apply(applied, argoManager)
		assert.ElementsMatch(t, []string{"dedicated", "node.example.com/spot"}, names(podList(t, sim.live(), "tolerations")),
			"SSA replaced the atomic tolerations list and removed the ignored entry")
	})
	t.Run("csa", func(t *testing.T) {
		out := csaApply(t, live, live, applied)
		assert.ElementsMatch(t, []string{"dedicated", "node.example.com/spot"}, names(podList(t, out, "tolerations")),
			"CSA removed the ignored toleration")
	})
}

// Ref: https://github.com/argoproj/argo-cd/issues/17694, https://github.com/argoproj/argo-cd/issues/23283.
func TestRespectIgnoreSemantics_TolerationChangeApplied_SiblingTolerationIgnored(t *testing.T) {
	live := toUnstructured(t, tolerationsLive)
	target := toUnstructured(t, tolerationsTarget)
	// The manifest also declares the spot toleration (a controller rewrites
	// its effect in-cluster, hence the ignore), and the user changes the
	// value of the non-ignored "dedicated" toleration.
	tols := podList(t, target, "tolerations")
	tols[0].(map[string]any)["value"] = "workers-v2"
	tols = append(tols, map[string]any{"key": "node.example.com/spot", "operator": "Exists", "effect": "NoSchedule"})
	require.NoError(t, unstructured.SetNestedSlice(target.Object, tols, "spec", "template", "spec", "tolerations"))
	liveTols := podList(t, live, "tolerations")
	liveTols[1].(map[string]any)["effect"] = "NoExecute"
	require.NoError(t, unstructured.SetNestedSlice(live.Object, liveTols, "spec", "template", "spec", "tolerations"))
	ignore := deploymentIgnore(`.spec.template.spec.tolerations[] | select(.key == "node.example.com/spot")`)

	applied := applyTarget(t, nil, live, target, ignore)

	require.Len(t, podList(t, applied, "tolerations"), 2)
	assert.Equal(t, "workers-v2", at(t, applied, "spec.template.spec.tolerations[0].value"),
		"the user's change to the non-ignored toleration is applied")
	assert.Equal(t, "NoExecute", at(t, applied, "spec.template.spec.tolerations[1].effect"),
		"the ignored toleration keeps its live effect")
}

// ---------------------------------------------------------------------------
// Use case 4: atomic map (nodeSelector) absent from the manifest, one key ignored.
// Granular maps can be applied partially; atomic maps cannot, so the apply
// target must not create a partial nodeSelector. Exercised for a built-in and
// for a CRD.
//
// Ref: https://github.com/argoproj/argo-cd/pull/29646 introduced key-level map
// recursion that emits the partial map; raised in its review. No public
// report of the shape yet.
// ---------------------------------------------------------------------------

const nodeSelectorTarget = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: batch
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: batch
  template:
    metadata:
      labels:
        app: batch
    spec:
      containers:
      - name: batch
        image: registry.example.com/batch:0.9.0
`

const nodeSelectorLive = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: batch
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: batch
  template:
    metadata:
      labels:
        app: batch
    spec:
      nodeSelector:
        kubernetes.io/os: linux
        topology.kubernetes.io/zone: us-east-1a
      containers:
      - name: batch
        image: registry.example.com/batch:0.9.0
`

func assertAtomicMapNotPartiallyAsserted(t *testing.T, applied *unstructured.Unstructured, wantLive map[string]string) {
	t.Helper()
	got, found, err := unstructured.NestedStringMap(applied.Object, "spec", "template", "spec", "nodeSelector")
	require.NoError(t, err)
	if found {
		// The manifest has no opinion about nodeSelector. If the apply target
		// asserts it at all it must assert the whole live map, because an
		// atomic map is replaced wholesale by whatever is applied.
		assert.Equal(t, wantLive, got, "apply target asserts a partial atomic map; SSA will replace the whole map with it")
	}
}

// Ref: https://github.com/argoproj/argo-cd/pull/29646.
func TestRespectIgnoreSemantics_InjectedNodeSelectorKeyIgnored_GitHasNoNodeSelector_Deployment(t *testing.T) {
	live := toUnstructured(t, nodeSelectorLive)
	target := toUnstructured(t, nodeSelectorTarget)
	// A scheduler add-on rewrites the zone key; a policy engine sets the OS
	// key. The manifest sets neither. The user ignores only the zone key.
	ignore := deploymentIgnore(`.spec.template.spec.nodeSelector["topology.kubernetes.io/zone"]`)
	wantLive := map[string]string{"kubernetes.io/os": "linux", "topology.kubernetes.io/zone": "us-east-1a"}

	applied := applyTarget(t, nil, live, target, ignore)
	assertAtomicMapNotPartiallyAsserted(t, applied, wantLive)

	// SSA: Argo CD never owned nodeSelector (the manifest has none; another
	// manager set it). Applying a partial map takes over the atomic field and
	// drops the other manager's key.
	t.Run("ssa", func(t *testing.T) {
		sim := newSSASim(t, builtinTypeConverter(t, deploymentGVK), deploymentGVK)
		sim.apply(target, argoManager)
		sim.update(live, otherManager)
		sim.apply(applied, argoManager)
		got, _, err := unstructured.NestedStringMap(sim.live().Object, "spec", "template", "spec", "nodeSelector")
		require.NoError(t, err)
		assert.Equal(t, wantLive, got, "SSA replaced the atomic nodeSelector map and removed a key Argo CD never managed")
	})
}

// Ref: https://github.com/argoproj/argo-cd/pull/29646.
func TestRespectIgnoreSemantics_InjectedNodeSelectorKeyIgnored_GitHasNoNodeSelector_RolloutCRD(t *testing.T) {
	const schemaPath = rolloutSchemaWithPatchMeta
	live := toUnstructured(t, nodeSelectorLive)
	target := toUnstructured(t, nodeSelectorTarget)
	for _, o := range []*unstructured.Unstructured{live, target} {
		o.SetAPIVersion("argoproj.io/v1alpha1")
		o.SetKind("Rollout")
	}
	ignore := rolloutIgnore(`.spec.template.spec.nodeSelector["topology.kubernetes.io/zone"]`)
	wantLive := map[string]string{"kubernetes.io/os": "linux", "topology.kubernetes.io/zone": "us-east-1a"}

	applied := applyTarget(t, gvkParserFromSchema(t, schemaPath), live, target, ignore)
	assertAtomicMapNotPartiallyAsserted(t, applied, wantLive)

	t.Run("ssa", func(t *testing.T) {
		sim := newSSASim(t, crdTypeConverter(t, schemaPath), rolloutGVK)
		sim.apply(target, argoManager)
		sim.update(live, otherManager)
		sim.apply(applied, argoManager)
		got, _, err := unstructured.NestedStringMap(sim.live().Object, "spec", "template", "spec", "nodeSelector")
		require.NoError(t, err)
		assert.Equal(t, wantLive, got, "SSA replaced the atomic nodeSelector map on the CRD and removed a key Argo CD never managed")
	})
}

// The CRD path for the atomic-list case, to show the OpenAPI patch meta does
// not change the outcome.
// Ref: https://github.com/argoproj/argo-cd/issues/29944.
func TestRespectIgnoreSemantics_InjectedTolerationSurvivesSync_RolloutCRD(t *testing.T) {
	const schemaPath = rolloutSchemaWithPatchMeta
	live := toUnstructured(t, tolerationsLive)
	target := toUnstructured(t, tolerationsTarget)
	for _, o := range []*unstructured.Unstructured{live, target} {
		o.SetAPIVersion("argoproj.io/v1alpha1")
		o.SetKind("Rollout")
	}
	ignore := rolloutIgnore(`.spec.template.spec.tolerations[] | select(.key == "node.example.com/spot")`)

	applied := applyTarget(t, gvkParserFromSchema(t, schemaPath), live, target, ignore)
	assert.ElementsMatch(t, []string{"dedicated", "node.example.com/spot"}, names(podList(t, applied, "tolerations")),
		"ignored toleration was dropped from the apply target")

	t.Run("ssa", func(t *testing.T) {
		sim := newSSASim(t, crdTypeConverter(t, schemaPath), rolloutGVK)
		sim.apply(target, argoManager)
		sim.update(live, otherManager)
		sim.apply(applied, argoManager)
		assert.ElementsMatch(t, []string{"dedicated", "node.example.com/spot"}, names(podList(t, sim.live(), "tolerations")))
	})
}

// ---------------------------------------------------------------------------
// Use case 5: Secret rendered with stringData, only the data key ignored. This is
// how the reporter of #29644 naturally wrote the ignore. The API server folds
// stringData into data (stringData wins on conflict), so the apply target has
// to be built against the folded object or the git stringData value overrides
// the ignored live data value.
//
// Ref: https://github.com/argoproj/argo-cd/issues/29644 (ignored data keys
// deleted, fixed by #29646) and https://github.com/argoproj/argo-cd/issues/12396
// (open since 2023: a helm randAlpha password keeps rotating despite the
// /data/password ignore, because stringData from git overrides it).
// ---------------------------------------------------------------------------

const secretTarget = `
apiVersion: v1
kind: Secret
metadata:
  name: db-credentials
  namespace: default
type: Opaque
stringData:
  username: app
  password: placeholder-from-git
`

const secretLive = `
apiVersion: v1
kind: Secret
metadata:
  name: db-credentials
  namespace: default
type: Opaque
data:
  username: YXBw
  password: cm90YXRlZC1ieS1vcGVyYXRvcg==
`

// Ref: https://github.com/argoproj/argo-cd/issues/29644, https://github.com/argoproj/argo-cd/issues/12396.
func TestRespectIgnoreSemantics_SecretPasswordIgnored_GitUsesStringData(t *testing.T) {
	live := toUnstructured(t, secretLive)
	target := toUnstructured(t, secretTarget)
	ignore := v1alpha1.ResourceIgnoreDifferences{Kind: "Secret", JQPathExpressions: []string{`.data["password"]`}}

	applied := applyTarget(t, nil, live, target, ignore)

	// The ignored key must end up with its live value after the API server
	// merges stringData over data. That requires the apply target to not
	// also carry the git value under stringData.password.
	data, _, err := unstructured.NestedStringMap(applied.Object, "data")
	require.NoError(t, err)
	stringData, _, err := unstructured.NestedStringMap(applied.Object, "stringData")
	require.NoError(t, err)
	assert.Equal(t, "cm90YXRlZC1ieS1vcGVyYXRvcg==", data["password"], "ignored password must be carried from live")
	_, inStringData := stringData["password"]
	assert.False(t, inStringData, "apply target still sets stringData.password from git; the API server will let it override the ignored data.password")
	// The non-ignored key must come from git, in either representation the
	// server accepts (stringData, or already folded into data).
	if u, inStringData := stringData["username"]; inStringData {
		assert.Equal(t, "app", u, "non-ignored key must come from git")
	} else {
		assert.Equal(t, "YXBw", data["username"], "non-ignored key must come from git")
	}
}

// ---------------------------------------------------------------------------
// Use case 6: known-types normalization must not leak into the apply target. The
// diff normalizer canonicalizes quantities for Rollout pod specs by default
// (and for any resource with a knownTypeFields override). CRDs store
// quantities verbatim, so a non-canonical value written to git earlier (1000m)
// stays non-canonical in live. Before this change, every field the normalizer
// changed on live was treated as ignored, so a later git change (2000m) was
// replaced by the live value. No ignore rule for the Rollout is needed to
// trigger it.
//
// Ref: https://github.com/argoproj/argo-cd/issues/23283#issuecomment-5758898568
// and the 2026-09-22 comment on that issue (non-canonical cpu values, no
// ignore rules), then https://github.com/argoproj/argo-cd/pull/29783.
// ---------------------------------------------------------------------------

const quantityRolloutTarget = `
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: api
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: api
  template:
    metadata:
      labels:
        app: api
    spec:
      containers:
      - name: api
        image: registry.example.com/api:2.4.0
        resources:
          requests:
            cpu: 2000m
            memory: 2048Mi
`

const quantityRolloutLive = `
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: api
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: api
  template:
    metadata:
      labels:
        app: api
    spec:
      containers:
      - name: api
        image: registry.example.com/api:2.4.0
        resources:
          requests:
            cpu: 1000m
            memory: 1024Mi
`

// Ref: https://github.com/argoproj/argo-cd/issues/23283 (2026-09-22 comment), https://github.com/argoproj/argo-cd/pull/29783.
func TestRespectIgnoreSemantics_CPURequestChangeApplied_NonCanonicalLiveValue(t *testing.T) {
	const schemaPath = rolloutSchemaWithPatchMeta
	// An unrelated, common ignore: replicas managed by an HPA.
	ignore := rolloutIgnore(`.spec.replicas`)

	assertRequests := func(t *testing.T, applied *unstructured.Unstructured) {
		t.Helper()
		containers := podList(t, applied, "containers")
		require.Len(t, containers, 1)
		requests, _, err := unstructured.NestedStringMap(containers[0].(map[string]any), "resources", "requests")
		require.NoError(t, err)
		for field, want := range map[string]string{"cpu": "2000m", "memory": "2048Mi"} {
			got, found := requests[field]
			require.True(t, found, "%s missing from apply target", field)
			wantQ, gotQ := resource.MustParse(want), resource.MustParse(got)
			assert.Zero(t, wantQ.Cmp(gotQ),
				"%s: git changed it to %s, apply target carries the live value %s", field, want, got)
		}
	}

	t.Run("with CRD schema", func(t *testing.T) {
		live := toUnstructured(t, quantityRolloutLive)
		target := toUnstructured(t, quantityRolloutTarget)
		assertRequests(t, applyTarget(t, gvkParserFromSchema(t, schemaPath), live, target, ignore))
	})
	t.Run("without schema", func(t *testing.T) {
		live := toUnstructured(t, quantityRolloutLive)
		target := toUnstructured(t, quantityRolloutTarget)
		assertRequests(t, applyTarget(t, builtinGVKParser(t), live, target, ignore))
	})
	t.Run("canonical value in git is unaffected", func(t *testing.T) {
		// Control: git already holds the canonical form. This always worked
		// and must keep working.
		live := toUnstructured(t, quantityRolloutLive)
		target := toUnstructured(t, quantityRolloutTarget)
		containers := podList(t, target, "containers")
		require.NoError(t, unstructured.SetNestedField(containers[0].(map[string]any), "2", "resources", "requests", "cpu"))
		require.NoError(t, unstructured.SetNestedField(containers[0].(map[string]any), "2Gi", "resources", "requests", "memory"))
		require.NoError(t, unstructured.SetNestedSlice(target.Object, containers, "spec", "template", "spec", "containers"))
		assertRequests(t, applyTarget(t, gvkParserFromSchema(t, schemaPath), live, target, ignore))
	})
}

// ---------------------------------------------------------------------------
// Use case 7: known-types normalization without any quantity. The known-types
// normalizer round-trips the pod spec through corev1.PodSpec with
// encoding/json, which emits `resources: {}` for every container that has no
// resources (ResourceRequirements is a struct, so omitempty does not apply).
// Live and normalized live therefore differ inside `containers` on almost
// every Rollout. Before this change that difference was treated as ignored and
// the whole live container list was copied over the target, losing the git
// change to an env var. This is the shape of #26588 as reported: no
// quantities, no ignore rule for the Rollout.
//
// Ref: https://github.com/argoproj/argo-cd/issues/26588 and the 2026-03-23
// comment on https://github.com/argoproj/argo-cd/pull/26924 (also reproduces
// with an unrelated Deployment rule configured).
// ---------------------------------------------------------------------------

const envRolloutTarget = `
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: demo-rollout
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: demo-rollout
  template:
    metadata:
      labels:
        app: demo-rollout
    spec:
      containers:
      - name: demo
        image: nginx:1.25
        ports:
        - name: http
          containerPort: 80
          protocol: TCP
        env:
        - name: ENV-VAR
          value: value-2
`

const envRolloutLive = `
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: demo-rollout
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: demo-rollout
  template:
    metadata:
      labels:
        app: demo-rollout
    spec:
      containers:
      - name: demo
        image: nginx:1.25
        ports:
        - name: http
          containerPort: 80
          protocol: TCP
        env:
        - name: ENV-VAR
          value: value-1
`

// Ref: https://github.com/argoproj/argo-cd/issues/26588.
func TestRespectIgnoreSemantics_RolloutEnvChangeApplied_NoRuleInvolved(t *testing.T) {
	assertEnv := func(t *testing.T, applied *unstructured.Unstructured) {
		t.Helper()
		assert.Equal(t, "value-2", at(t, applied, "spec.template.spec.containers[0].env[0].value"),
			"git changed ENV-VAR to value-2; the apply target carries the live value and the app stays OutOfSync")
	}

	t.Run("no ignore rules at all", func(t *testing.T) {
		live, target := toUnstructured(t, envRolloutLive), toUnstructured(t, envRolloutTarget)
		assertEnv(t, applyTargetWith(t, gvkParserFromSchema(t, rolloutSchemaRealistic), live, target, applyOpts{}))
	})
	t.Run("unrelated rule for another kind", func(t *testing.T) {
		live, target := toUnstructured(t, envRolloutLive), toUnstructured(t, envRolloutTarget)
		ignore := v1alpha1.ResourceIgnoreDifferences{Group: "apps", Kind: "Deployment", JSONPointers: []string{"/spec/replicas"}}
		assertEnv(t, applyTarget(t, gvkParserFromSchema(t, rolloutSchemaRealistic), live, target, ignore))
	})
	t.Run("without schema", func(t *testing.T) {
		live, target := toUnstructured(t, envRolloutLive), toUnstructured(t, envRolloutTarget)
		assertEnv(t, applyTargetWith(t, builtinGVKParser(t), live, target, applyOpts{}))
	})
}

// ---------------------------------------------------------------------------
// Use case 8: CRD list with list-map keys but no legacy patch-merge-key,
// field-level ignore inside an element that exists on both sides, sibling
// field changed in git. This is #23283 as originally reported: image and
// imagePullPolicy ignored on a Rollout, resources bumped in git, the bump
// never applied. Quantities are canonical here, so this isolates the list
// handling from case 6.
//
// Ref: https://github.com/argoproj/argo-cd/issues/23283 (original report and
// the 2026-09-18 reproduction on v3.6.0-rc1), fix attempt
// https://github.com/argoproj/argo-cd/pull/29783.
// ---------------------------------------------------------------------------

const imageIgnoreRolloutTarget = `
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: nginx-rollout
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:latest
        imagePullPolicy: Always
        ports:
        - containerPort: 80
          protocol: TCP
        resources:
          requests:
            cpu: 28m
            memory: 80Mi
          limits:
            cpu: 500m
            memory: 256Mi
`

const imageIgnoreRolloutLive = `
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: nginx-rollout
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.23
        imagePullPolicy: Always
        ports:
        - containerPort: 80
          protocol: TCP
        resources:
          requests:
            cpu: 25m
            memory: 70Mi
          limits:
            cpu: 500m
            memory: 256Mi
`

// Ref: https://github.com/argoproj/argo-cd/issues/23283.
func TestRespectIgnoreSemantics_RolloutImageIgnored_ResourcesChangeApplied(t *testing.T) {
	live := toUnstructured(t, imageIgnoreRolloutLive)
	target := toUnstructured(t, imageIgnoreRolloutTarget)
	ignore := rolloutIgnore(`.spec.template.spec.containers[].image`, `.spec.template.spec.containers[].imagePullPolicy`)

	applied := applyTarget(t, gvkParserFromSchema(t, rolloutSchemaRealistic), live, target, ignore)

	containers := podList(t, applied, "containers")
	require.Len(t, containers, 1)
	c := containers[0].(map[string]any)
	requests, _, err := unstructured.NestedStringMap(c, "resources", "requests")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"cpu": "28m", "memory": "80Mi"}, requests,
		"git bumped the requests; the apply target carries the live values and the Rollout stays OutOfSync")
	// The ignored field must take the live value (CI pushed the drifted image).
	assert.Equal(t, "nginx:1.23", c["image"], "ignored image must be carried from live")
}

// ---------------------------------------------------------------------------
// Use case 9: ignored live-only item in a list of unnamed items must keep its live
// position. Argo Rollouts inserts a header-route path in front of the stable
// path; ALB rule priority follows path order, so appending it last is still a
// broken canary. This is the exact object from #29944.
//
// Ref: https://github.com/argoproj/argo-cd/issues/29944 and the fix attempt
// https://github.com/argoproj/argo-cd/pull/29945.
// ---------------------------------------------------------------------------

const managedRouteIngressLive = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app
  namespace: default
spec:
  ingressClassName: alb
  rules:
  - http:
      paths:
      - path: /*
        pathType: ImplementationSpecific
        backend:
          service:
            name: canary-header-route
            port:
              name: use-annotation
      - path: /*
        pathType: ImplementationSpecific
        backend:
          service:
            name: app-stable
            port:
              name: use-annotation
`

const managedRouteIngressTarget = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app
  namespace: default
spec:
  ingressClassName: alb
  rules:
  - http:
      paths:
      - path: /*
        pathType: ImplementationSpecific
        backend:
          service:
            name: app-stable
            port:
              name: use-annotation
`

func ingressBackendNames(t *testing.T, u *unstructured.Unstructured) []string {
	t.Helper()
	rules, _, err := unstructured.NestedSlice(u.Object, "spec", "rules")
	require.NoError(t, err)
	var out []string
	for _, r := range rules {
		paths, _, _ := unstructured.NestedSlice(r.(map[string]any), "http", "paths")
		for _, p := range paths {
			n, _, _ := unstructured.NestedString(p.(map[string]any), "backend", "service", "name")
			out = append(out, n)
		}
	}
	return out
}

// Ref: https://github.com/argoproj/argo-cd/issues/29944.
func TestRespectIgnoreSemantics_RolloutsManagedIngressRouteSurvivesSync(t *testing.T) {
	live := toUnstructured(t, managedRouteIngressLive)
	target := toUnstructured(t, managedRouteIngressTarget)
	ignore := v1alpha1.ResourceIgnoreDifferences{
		Group: "networking.k8s.io", Kind: "Ingress",
		JQPathExpressions: []string{`.spec.rules[].http.paths[] | select(.backend.service.name == "canary-header-route")`},
	}

	applied := applyTarget(t, nil, live, target, ignore)

	// Order is part of the assertion on purpose.
	assert.Equal(t, []string{"canary-header-route", "app-stable"}, ingressBackendNames(t, applied),
		"the injected header route must survive at its live position")

	// paths is an atomic list: whoever applies it owns the whole list, so the
	// apply target must carry the injected route in its live position.
	t.Run("ssa", func(t *testing.T) {
		sim := newSSASim(t, builtinTypeConverter(t, ingressGVK), ingressGVK)
		sim.apply(target, argoManager)
		sim.update(live, otherManager) // Argo Rollouts inserts the header route
		sim.apply(applied, argoManager)
		assert.Equal(t, []string{"canary-header-route", "app-stable"}, ingressBackendNames(t, sim.live()),
			"SSA removed or reordered the header route mid-canary")
	})
	t.Run("csa", func(t *testing.T) {
		out := csaApply(t, live, live, applied)
		assert.Equal(t, []string{"canary-header-route", "app-stable"}, ingressBackendNames(t, out),
			"CSA removed or reordered the header route mid-canary")
	})
}

// ---------------------------------------------------------------------------
// Use case 10: rule inside a list of unnamed items that exist on both sides, on a
// CRD (child Application in app-of-apps). `spec.sources` has no merge key, so
// items are matched by position: the ignored field of sources[1] takes its
// live value and the git change to sources[0] is applied. Covers the
// index-based and the select-based spelling of the rule.
//
// Ref: https://github.com/argoproj/argo-cd/issues/17694 (and its 2026-05-25
// comment with this exact multi-source shape),
// https://github.com/argoproj/argo-cd/issues/24880,
// https://github.com/argoproj/argo-cd/issues/25855, fix attempt
// https://github.com/argoproj/argo-cd/pull/29674.
// ---------------------------------------------------------------------------

const childAppTarget = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: helm-child
  namespace: argocd
spec:
  project: default
  destination:
    server: https://kubernetes.default.svc
    namespace: default
  sources:
  - repoURL: https://git.example.com/python-app-helm-chart.git
    targetRevision: chart_v2
    path: .
    helm:
      releaseName: python-app
      valueFiles:
      - $values/values.yaml
  - repoURL: https://git.example.com/python-app-helm-values.git
    targetRevision: values_v1
    ref: values
`

const childAppLive = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: helm-child
  namespace: argocd
spec:
  project: default
  destination:
    server: https://kubernetes.default.svc
    namespace: default
  sources:
  - repoURL: https://git.example.com/python-app-helm-chart.git
    targetRevision: chart_v1
    path: .
    helm:
      releaseName: python-app
      valueFiles:
      - $values/values.yaml
  - repoURL: https://git.example.com/python-app-helm-values.git
    targetRevision: values_v2
    ref: values
`

// Ref: https://github.com/argoproj/argo-cd/issues/17694, https://github.com/argoproj/argo-cd/issues/24880.
func TestRespectIgnoreSemantics_ChildAppValuesRevisionIgnored_ChartRevisionChangeApplied(t *testing.T) {
	appIgnore := func(jq []string, ptrs []string) v1alpha1.ResourceIgnoreDifferences {
		return v1alpha1.ResourceIgnoreDifferences{Group: "argoproj.io", Kind: "Application", JQPathExpressions: jq, JSONPointers: ptrs}
	}
	rules := map[string]v1alpha1.ResourceIgnoreDifferences{
		"index-based jq (#24880)":      appIgnore([]string{`.spec.sources[1].targetRevision`}, nil),
		"index-based jsonPointer":      appIgnore(nil, []string{"/spec/sources/1/targetRevision"}),
		"selected by repoURL (#17694)": appIgnore([]string{`.spec.sources[] | select(.repoURL == "https://git.example.com/python-app-helm-values.git") | .targetRevision`}, nil),
	}
	for name, ignore := range rules {
		t.Run(name, func(t *testing.T) {
			live := toUnstructured(t, childAppLive)
			target := toUnstructured(t, childAppTarget)

			applied := applyTarget(t, gvkParserFromSchema(t, applicationSchema), live, target, ignore)

			assert.Equal(t, "chart_v2", at(t, applied, "spec.sources[0].targetRevision"),
				"git's change to the chart revision is applied")
			assert.Equal(t, "values_v2", at(t, applied, "spec.sources[1].targetRevision"),
				"the ignored values revision keeps its live value")
		})
	}
}

// ---------------------------------------------------------------------------
// Use case 11: the resource-tracking migration is not an ignored field. Live
// still carries only the legacy app.kubernetes.io/instance label; the target
// carries the tracking-id annotation. The tracking normalizer rewrites live
// (adds the annotation, removes the label) before the diff. Before this change
// that rewrite was treated like an ignore: #27600 reported the annotation
// being stripped from the target, and later the legacy label was copied from
// live onto the apply target, so every sync re-applied the label the
// migration is supposed to remove.
//
// Ref: https://github.com/argoproj/argo-cd/pull/27600 (test case and analysis),
// https://github.com/argoproj/argo-cd/issues/23334, https://github.com/argoproj/argo-cd/issues/23687.
// ---------------------------------------------------------------------------

const trackingRolloutLive = `
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: guestbook
  namespace: default
  labels:
    app.kubernetes.io/instance: guestbook
spec:
  replicas: 5
  selector:
    matchLabels:
      app: guestbook
  template:
    metadata:
      labels:
        app: guestbook
    spec:
      containers:
      - name: guestbook
        image: quay.io/argoprojlabs/argocd-e2e-container:0.1
`

const trackingRolloutTarget = `
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: guestbook
  namespace: default
  annotations:
    argocd.argoproj.io/tracking-id: 'guestbook:argoproj.io/Rollout:default/guestbook'
spec:
  replicas: 1
  selector:
    matchLabels:
      app: guestbook
  template:
    metadata:
      labels:
        app: guestbook
    spec:
      containers:
      - name: guestbook
        image: quay.io/argoprojlabs/argocd-e2e-container:0.1
`

// Ref: https://github.com/argoproj/argo-cd/pull/27600.
func TestRespectIgnoreSemantics_TrackingLabelMigrationDoesNotBlockSpecChange(t *testing.T) {
	ignore := v1alpha1.ResourceIgnoreDifferences{Group: "argoproj.io", Kind: "Rollout", JSONPointers: []string{"/spec/replicas"}}
	check := func(t *testing.T, parser *managedfields.GvkParser) {
		t.Helper()
		live := toUnstructured(t, trackingRolloutLive)
		target := toUnstructured(t, trackingRolloutTarget)
		applied := applyTargetWith(t, parser, live, target, applyOpts{ignores: []v1alpha1.ResourceIgnoreDifferences{ignore}, tracking: true})

		assert.Equal(t, "guestbook:argoproj.io/Rollout:default/guestbook", applied.GetAnnotations()[common.AnnotationKeyAppInstance],
			"tracking-id annotation was stripped: the tracking migration on live was treated as an ignored field")
		assert.NotContains(t, applied.GetLabels(), common.LabelKeyAppInstance,
			"the legacy instance label was copied from live onto the apply target: the tracking migration on live was treated as an ignored field, so the label is re-applied on every sync")
		replicas, _, err := unstructured.NestedInt64(applied.Object, "spec", "replicas")
		require.NoError(t, err)
		assert.Equal(t, int64(5), replicas, "the ignored replicas must take the live value")
	}
	t.Run("with CRD schema", func(t *testing.T) { check(t, gvkParserFromSchema(t, rolloutSchemaRealistic)) })
	t.Run("without schema", func(t *testing.T) { check(t, builtinGVKParser(t)) })
}

// ---------------------------------------------------------------------------
// Use case 12: a conditional rule that matches the live value but not the git
// value. The diff compares git's `desired` against a live object with the
// field stripped and reports OutOfSync, so the sync must apply `desired`:
// git wins wherever it still asserts a value. Keeping live whenever the rule
// matched live would leave the app OutOfSync forever.
//
// Ref: https://github.com/argoproj/argo-cd/pull/29945 (Qodo finding 1).
// ---------------------------------------------------------------------------

const conditionalCMTarget = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: settings
  namespace: default
data:
  setting: desired
  other: same
`

const conditionalCMLive = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: settings
  namespace: default
data:
  setting: live
  other: same
`

// Ref: https://github.com/argoproj/argo-cd/pull/29945.
func TestRespectIgnoreSemantics_ConditionalRuleMatchesOnlyLive_GitValueApplied(t *testing.T) {
	live := toUnstructured(t, conditionalCMLive)
	target := toUnstructured(t, conditionalCMTarget)
	ignore := v1alpha1.ResourceIgnoreDifferences{Kind: "ConfigMap", JQPathExpressions: []string{`.data.setting | select(. == "live")`}}

	applied := applyTarget(t, nil, live, target, ignore)

	data, _, err := unstructured.NestedStringMap(applied.Object, "data")
	require.NoError(t, err)
	assert.Equal(t, "desired", data["setting"],
		"the diff reports OutOfSync for this field, so the apply target must carry git's value")
	assert.Equal(t, "same", data["other"])
}

// ---------------------------------------------------------------------------
// Cases 18+: the rule shapes most often reported by users, collected from
// GitHub issues, discussions and third-party guides. Each one names its sources.
// ---------------------------------------------------------------------------

// Use case 13: webhook caBundle. After replicas this is the most common rule: the
// value is written into live by cert-manager's cainjector or the operator
// itself, and git has no value at all. `webhooks` is a merge-keyed list (name);
// the live-only scalar must be carried into the matching item while a sibling
// change from git (failurePolicy) is applied.
//
// Ref: docs/user-guide/diffing.md ("System-Level Configuration" example),
// https://github.com/argoproj/argo-cd/issues/9323,
// https://github.com/argoproj/argo-cd/issues/12961,
// https://github.com/argoproj/argo-cd/issues/25157,
// https://github.com/argoproj/argo-cd/issues/4276 (positional pointer form),
// https://documentation.suse.com/cloudnative/admission-controller/dev/en/howtos/argocd-installation.html.
const webhookTarget = `
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: injector
webhooks:
- name: inject.example.com
  admissionReviewVersions: ["v1"]
  sideEffects: None
  failurePolicy: Fail
  clientConfig:
    service: {name: injector, namespace: default, path: /mutate}
`

const webhookLive = `
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: injector
webhooks:
- name: inject.example.com
  admissionReviewVersions: ["v1"]
  sideEffects: None
  failurePolicy: Ignore
  clientConfig:
    caBundle: QUJDREVG
    service: {name: injector, namespace: default, path: /mutate}
`

func TestRespectIgnoreSemantics_InjectedCABundleSurvivesSync_FailurePolicyChangeApplied(t *testing.T) {
	target := toUnstructured(t, webhookTarget)
	live := toUnstructured(t, webhookLive)
	check := func(t *testing.T, ignore v1alpha1.ResourceIgnoreDifferences) {
		t.Helper()
		applied := applyTarget(t, nil, live, target, ignore)
		assert.Equal(t, "Fail", at(t, applied, "webhooks[name=inject.example.com].failurePolicy"), "git's change to a sibling field is applied")
		assert.Equal(t, "QUJDREVG", at(t, applied, "webhooks[name=inject.example.com].clientConfig.caBundle"), "the injected caBundle is kept")

		sim := newSSASim(t, builtinTypeConverter(t, webhookGVK), webhookGVK)
		sim.apply(target, argoManager)
		sim.update(live, otherManager) // cainjector writes the caBundle, someone edits failurePolicy
		sim.apply(applied, argoManager)
		synced := sim.live()
		assert.Equal(t, "Fail", at(t, synced, "webhooks[name=inject.example.com].failurePolicy"))
		assert.Equal(t, "QUJDREVG", at(t, synced, "webhooks[name=inject.example.com].clientConfig.caBundle"))
	}
	rule := func(exprs ...string) v1alpha1.ResourceIgnoreDifferences {
		return v1alpha1.ResourceIgnoreDifferences{Group: webhookGVK.Group, Kind: webhookGVK.Kind, JQPathExpressions: exprs}
	}
	t.Run("jq with optional iterator", func(t *testing.T) { check(t, rule(`.webhooks[]?.clientConfig.caBundle`)) })
	t.Run("jq", func(t *testing.T) { check(t, rule(`.webhooks[].clientConfig.caBundle`)) })
	t.Run("positional pointer", func(t *testing.T) {
		check(t, v1alpha1.ResourceIgnoreDifferences{Group: webhookGVK.Group, Kind: webhookGVK.Kind, JSONPointers: []string{"/webhooks/0/clientConfig/caBundle"}})
	})
}

// Use case 14: an annotation added to the pod template at runtime
// (`kubectl rollout restart` writes kubectl.kubernetes.io/restartedAt) while
// git declares no annotations map at all. The granular map is created to hold
// the carried key. Covers the `~1` pointer escape and the `?.["key"]` jq form.
//
// Ref: https://github.com/argoproj/argo-cd/issues/7513,
// https://github.com/argoproj/argo-cd/issues/19834 (GKE annotations, `?.[...]` form),
// docs/user-guide/diffing.md (`~1` escape).
const restartedAtTarget = `
apiVersion: apps/v1
kind: Deployment
metadata: {name: web, namespace: default}
spec:
  selector: {matchLabels: {app: web}}
  template:
    metadata: {labels: {app: web}}
    spec:
      containers:
      - {name: web, image: nginx:1.27}
`

const restartedAtLive = `
apiVersion: apps/v1
kind: Deployment
metadata: {name: web, namespace: default}
spec:
  selector: {matchLabels: {app: web}}
  template:
    metadata:
      labels: {app: web}
      annotations: {kubectl.kubernetes.io/restartedAt: "2026-10-30T10:00:00Z"}
    spec:
      containers:
      - {name: web, image: nginx:1.26}
`

func TestRespectIgnoreSemantics_RolloutRestartAnnotationSurvivesSync_GitHasNoAnnotations(t *testing.T) {
	target := toUnstructured(t, restartedAtTarget)
	live := toUnstructured(t, restartedAtLive)
	check := func(t *testing.T, ignore v1alpha1.ResourceIgnoreDifferences) {
		t.Helper()
		applied := applyTarget(t, nil, live, target, ignore)
		ann, _, err := unstructured.NestedStringMap(applied.Object, "spec", "template", "metadata", "annotations")
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"kubectl.kubernetes.io/restartedAt": "2026-10-30T10:00:00Z"}, ann)

		sim := newSSASim(t, builtinTypeConverter(t, deploymentGVK), deploymentGVK)
		sim.apply(target, argoManager)
		sim.update(live, otherManager)
		sim.apply(applied, argoManager)
		out := sim.live()
		ann, _, err = unstructured.NestedStringMap(out.Object, "spec", "template", "metadata", "annotations")
		require.NoError(t, err)
		assert.Equal(t, "2026-10-30T10:00:00Z", ann["kubectl.kubernetes.io/restartedAt"], "the restart annotation survives the sync")
		assert.Equal(t, "nginx:1.27", at(t, out, "spec.template.spec.containers[0].image"), "git's image change is applied")
	}
	t.Run("pointer", func(t *testing.T) {
		check(t, v1alpha1.ResourceIgnoreDifferences{Group: "apps", Kind: "Deployment", JSONPointers: []string{"/spec/template/metadata/annotations/kubectl.kubernetes.io~1restartedAt"}})
	})
	t.Run("jq", func(t *testing.T) {
		check(t, deploymentIgnore(`.spec.template.metadata.annotations["kubectl.kubernetes.io/restartedAt"]`))
	})
	t.Run("jq with optional access", func(t *testing.T) {
		check(t, deploymentIgnore(`.spec.template.metadata.annotations?.["kubectl.kubernetes.io/restartedAt"]`))
	})
}

// Use case 15: merge-keyed lists ordered differently in git and live, with a rule
// inside a nested keyed list (env). The carried value must follow the item by
// its key, not its index, at both levels.
//
// Also pins a jq property users hit constantly: `.env[]` fails with "cannot
// iterate over null" on a container that has no env, and a failing expression
// contributes nothing (the same happens at diff time). `.env[]?` is the form
// that works, and the one the reports use.
//
// Ref: https://github.com/argoproj/argo-cd/issues/18197 (env rule),
// https://github.com/argoproj/argo-cd/issues/24275 (`[]?` form).
const reorderedTarget = `
apiVersion: apps/v1
kind: Deployment
metadata: {name: web, namespace: default}
spec:
  selector: {matchLabels: {app: web}}
  template:
    metadata: {labels: {app: web}}
    spec:
      containers:
      - name: web
        image: nginx:1.27
        env:
        - {name: A, value: a-git}
        - {name: B, value: b-git}
      - name: sidecar
        image: sidecar:1
`

const reorderedLive = `
apiVersion: apps/v1
kind: Deployment
metadata: {name: web, namespace: default}
spec:
  selector: {matchLabels: {app: web}}
  template:
    metadata: {labels: {app: web}}
    spec:
      containers:
      - name: sidecar
        image: sidecar:1
        imagePullPolicy: IfNotPresent
      - name: web
        image: nginx:1.27
        imagePullPolicy: Always
        env:
        - {name: B, value: b-live}
        - {name: A, value: a-git}
`

func TestRespectIgnoreSemantics_EnvVarIgnored_ListsOrderedDifferentlyInGitAndLive(t *testing.T) {
	target := toUnstructured(t, reorderedTarget)
	live := toUnstructured(t, reorderedLive)
	pullPolicyRule := `.spec.template.spec.containers[] | select(.name == "web") | .imagePullPolicy`

	t.Run("values follow the item by key", func(t *testing.T) {
		applied := applyTarget(t, nil, live, target, deploymentIgnore(pullPolicyRule,
			`.spec.template.spec.containers[].env[]? | select(.name == "B") | .value`))
		assert.Equal(t, []string{"web", "sidecar"}, names(podList(t, applied, "containers")), "git's container order is kept")
		assert.Equal(t, "Always", at(t, applied, "spec.template.spec.containers[name=web].imagePullPolicy"),
			"web's live pull policy is kept although live lists web at another index")
		absent(t, applied, "spec.template.spec.containers[name=sidecar].imagePullPolicy")
		assert.Equal(t, []string{"A", "B"}, names(at(t, applied, "spec.template.spec.containers[name=web].env").([]any)), "git's env order is kept")
		assert.Equal(t, "b-live", at(t, applied, "spec.template.spec.containers[name=web].env[name=B].value"), "the ignored env var keeps its live value")
		assert.Equal(t, "a-git", at(t, applied, "spec.template.spec.containers[name=web].env[name=A].value"))
	})

	t.Run("unguarded iterator over a missing field disables the rule", func(t *testing.T) {
		applied := applyTarget(t, nil, live, target, deploymentIgnore(pullPolicyRule,
			`.spec.template.spec.containers[].env[] | select(.name == "B") | .value`))
		assert.Equal(t, "Always", at(t, applied, "spec.template.spec.containers[name=web].imagePullPolicy"), "the other rule still works")
		assert.Equal(t, "b-git", at(t, applied, "spec.template.spec.containers[name=web].env[name=B].value"),
			"the env rule errors on the sidecar (no env) and is skipped, as it is at diff time")
	})
}

// Use case 16: a whole list ignored by pointer on a CRD. The live list replaces
// git's as one value, whatever the schema says about the list. Reported for
// ApplicationSet /spec/generators, where the sync emptied the list.
//
// Ref: https://github.com/argoproj/argo-cd/issues/17949.
const wholeListTarget = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: child, namespace: argocd}
spec:
  project: default
  destination: {server: https://kubernetes.default.svc, namespace: default}
  sources:
  - {repoURL: https://example.com/a.git, path: a, targetRevision: main}
`

const wholeListLive = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: child, namespace: argocd}
spec:
  project: default
  destination: {server: https://kubernetes.default.svc, namespace: default}
  sources:
  - {repoURL: https://example.com/a.git, path: a, targetRevision: v2, helm: {parameters: [{name: image.tag, value: "1.2.3"}]}}
  - {repoURL: https://example.com/b.git, path: b, targetRevision: main}
`

func TestRespectIgnoreSemantics_WholeListIgnored_LiveListKept_CRD(t *testing.T) {
	target := toUnstructured(t, wholeListTarget)
	live := toUnstructured(t, wholeListLive)
	ignore := v1alpha1.ResourceIgnoreDifferences{Group: "argoproj.io", Kind: "Application", JSONPointers: []string{"/spec/sources"}}
	check := func(t *testing.T, parser *managedfields.GvkParser) {
		t.Helper()
		applied := applyTarget(t, parser, live, target, ignore)
		assert.Len(t, at(t, applied, "spec.sources"), 2, "the live list is carried whole")
		assert.Equal(t, "v2", at(t, applied, "spec.sources[0].targetRevision"))
		assert.Equal(t, "1.2.3", at(t, applied, "spec.sources[0].helm.parameters[name=image.tag].value"))
		assert.Equal(t, "default", at(t, applied, "spec.project"), "the rest of spec is applied from git")
	}
	t.Run("with CRD schema", func(t *testing.T) { check(t, gvkParserFromSchema(t, applicationSchema)) })
	t.Run("without schema", func(t *testing.T) { check(t, builtinGVKParser(t)) })
}

// Use case 17: a Secret whose whole data is ignored because a chart generates a
// random password that must only be created once. Git writes it as stringData.
// The fold runs before the rules, so `/data` matches the folded manifest and
// a rule on `/stringData/...` matches nothing, exactly as at diff time (the
// diff folds stringData before running the ignore normalizer too).
//
// Ref: https://github.com/argoproj/argo-cd/issues/8810,
// https://docs.cilium.io/en/stable/configuration/argocd-issues/ (per-key data rules).
const randomPasswordTarget = `
apiVersion: v1
kind: Secret
metadata: {name: db, namespace: default}
type: Opaque
stringData: {password: git-password}
`

const randomPasswordLive = `
apiVersion: v1
kind: Secret
metadata: {name: db, namespace: default}
type: Opaque
data: {password: bGl2ZS1wYXNzd29yZA==}
`

func TestRespectIgnoreSemantics_SecretAllDataIgnored_GitUsesStringData(t *testing.T) {
	target := toUnstructured(t, randomPasswordTarget)
	live := toUnstructured(t, randomPasswordLive)

	t.Run("/data keeps the live password", func(t *testing.T) {
		applied := applyTarget(t, nil, live, target, v1alpha1.ResourceIgnoreDifferences{Kind: "Secret", JSONPointers: []string{"/data"}})
		data, _, err := unstructured.NestedStringMap(applied.Object, "data")
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"password": "bGl2ZS1wYXNzd29yZA=="}, data)
		assert.NotContains(t, applied.Object, "stringData")
	})

	t.Run("/stringData matches nothing", func(t *testing.T) {
		applied := applyTarget(t, nil, live, target, v1alpha1.ResourceIgnoreDifferences{Kind: "Secret", JSONPointers: []string{"/stringData/password"}})
		sd, _, err := unstructured.NestedStringMap(applied.Object, "stringData")
		require.NoError(t, err)
		assert.Equal(t, "git-password", sd["password"], "no path is ignored, so the manifest is applied as is")
	})
}

// Use case 18: tokens created through the UI inside AppProject roles, on a CRD
// with no schema (lists are positional). The live-only jwtTokens list is
// carried into git's role while git's policies change is applied.
//
// Ref: https://github.com/argoproj/argo-cd/issues/27480,
// https://github.com/argoproj/argo-cd/discussions/25178.
const appProjectTarget = `
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata: {name: p, namespace: argocd}
spec:
  roles:
  - name: ro
    policies: ["p, proj:p:ro, applications, get, p/*, allow", "p, proj:p:ro, applications, sync, p/*, allow"]
`

const appProjectLive = `
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata: {name: p, namespace: argocd}
spec:
  roles:
  - name: ro
    policies: ["p, proj:p:ro, applications, get, p/*, allow"]
    jwtTokens:
    - {iat: 1700000000, id: abc}
`

func TestRespectIgnoreSemantics_UICreatedTokensSurviveSync_PolicyChangeApplied(t *testing.T) {
	target := toUnstructured(t, appProjectTarget)
	live := toUnstructured(t, appProjectLive)
	ignore := v1alpha1.ResourceIgnoreDifferences{Group: "argoproj.io", Kind: "AppProject", JQPathExpressions: []string{`.spec.roles[].jwtTokens`}}

	applied := applyTarget(t, nil, live, target, ignore)

	roles, _, err := unstructured.NestedSlice(applied.Object, "spec", "roles")
	require.NoError(t, err)
	require.Len(t, roles, 1)
	role := roles[0].(map[string]any)
	assert.Len(t, role["policies"], 2, "git's new policy is applied")
	assert.Equal(t, []any{map[string]any{"iat": int64(1700000000), "id": "abc"}}, role["jwtTokens"], "the UI-created tokens are kept")
}

// Use case 19: Service fields the API server allocates. clusterIP is a scalar and
// clusterIPs an atomic list; both are ignored as a whole, so both are carried
// even though git declares neither. (Only a partial set of keys inside an
// undeclared atomic field is refused.)
//
// Ref: https://oneuptime.com/blog/post/2026-02-26-argocd-diff-customization-cheat-sheet/view.
const serviceTarget = `
apiVersion: v1
kind: Service
metadata: {name: svc, namespace: default}
spec:
  selector: {app: web}
  ports: [{name: http, port: 80, targetPort: 8080}]
`

const serviceLive = `
apiVersion: v1
kind: Service
metadata: {name: svc, namespace: default}
spec:
  selector: {app: web}
  clusterIP: 10.0.0.5
  clusterIPs: [10.0.0.5]
  ipFamilies: [IPv4]
  ipFamilyPolicy: SingleStack
  type: ClusterIP
  ports: [{name: http, port: 80, targetPort: 8080, protocol: TCP}]
`

func TestRespectIgnoreSemantics_AllocatedClusterIPsKept(t *testing.T) {
	target := toUnstructured(t, serviceTarget)
	live := toUnstructured(t, serviceLive)
	ignore := v1alpha1.ResourceIgnoreDifferences{Kind: "Service", JSONPointers: []string{"/spec/clusterIP", "/spec/clusterIPs"}}

	applied := applyTarget(t, nil, live, target, ignore)

	spec := applied.Object["spec"].(map[string]any)
	assert.Equal(t, "10.0.0.5", spec["clusterIP"])
	assert.Equal(t, []any{"10.0.0.5"}, spec["clusterIPs"])
	assert.NotContains(t, spec, "ipFamilies", "fields no rule names are not carried")
	assert.NotContains(t, spec, "type")
}

// Use case 20: server-defaulted fields inside StatefulSet volumeClaimTemplates,
// including a literal null (creationTimestamp) and a nested status. The list
// has no merge keys, so the item is matched by position; null is carried as
// null and server-side apply accepts it.
//
// Ref: https://github.com/argoproj/argo-cd/issues/24791,
// https://github.com/argoproj/argo-cd/issues/11143.
const statefulSetTarget = `
apiVersion: apps/v1
kind: StatefulSet
metadata: {name: db, namespace: default}
spec:
  serviceName: db
  selector: {matchLabels: {app: db}}
  template:
    metadata: {labels: {app: db}}
    spec:
      containers:
      - {name: db, image: postgres:17}
  volumeClaimTemplates:
  - metadata: {name: data}
    spec:
      accessModes: [ReadWriteOnce]
      resources: {requests: {storage: 1Gi}}
`

const statefulSetLive = `
apiVersion: apps/v1
kind: StatefulSet
metadata: {name: db, namespace: default}
spec:
  serviceName: db
  selector: {matchLabels: {app: db}}
  template:
    metadata: {labels: {app: db}}
    spec:
      containers:
      - {name: db, image: postgres:16}
  volumeClaimTemplates:
  - apiVersion: v1
    kind: PersistentVolumeClaim
    metadata: {name: data, creationTimestamp: null}
    spec:
      accessModes: [ReadWriteOnce]
      resources: {requests: {storage: 1Gi}}
      volumeMode: Filesystem
    status: {phase: Pending}
`

func TestRespectIgnoreSemantics_VolumeClaimTemplateDefaultsKept_ImageChangeApplied(t *testing.T) {
	target := toUnstructured(t, statefulSetTarget)
	live := toUnstructured(t, statefulSetLive)
	ignore := v1alpha1.ResourceIgnoreDifferences{Group: "apps", Kind: "StatefulSet", JQPathExpressions: []string{
		`.spec.volumeClaimTemplates[].apiVersion`,
		`.spec.volumeClaimTemplates[].kind`,
		`.spec.volumeClaimTemplates[].metadata.creationTimestamp`,
		`.spec.volumeClaimTemplates[].status`,
	}}

	applied := applyTarget(t, nil, live, target, ignore)

	vcts, _, err := unstructured.NestedSlice(applied.Object, "spec", "volumeClaimTemplates")
	require.NoError(t, err)
	require.Len(t, vcts, 1)
	vct := vcts[0].(map[string]any)
	assert.Equal(t, "v1", vct["apiVersion"])
	assert.Equal(t, "PersistentVolumeClaim", vct["kind"])
	meta := vct["metadata"].(map[string]any)
	assert.Contains(t, meta, "creationTimestamp")
	assert.Nil(t, meta["creationTimestamp"], "a live null is carried as null")
	assert.Equal(t, map[string]any{"phase": "Pending"}, vct["status"], "only top-level status is excluded")
	assert.NotContains(t, vct["spec"], "volumeMode", "a defaulted field no rule names is not carried")

	sim := newSSASim(t, builtinTypeConverter(t, statefulSetGVK), statefulSetGVK)
	sim.apply(target, argoManager)
	sim.update(live, otherManager)
	sim.apply(applied, argoManager)
	out := sim.live()
	assert.Equal(t, "postgres:17", at(t, out, "spec.template.spec.containers[0].image"))
}

// Use case 21: managedFieldsManagers, the server-side-apply-native rule: trust
// whatever kube-controller-manager owns. The HPA scales through the scale
// subresource, so kube-controller-manager owns spec.replicas in live. The
// live value is carried over git's while git's image change is applied.
//
// The second subtest pins an open limitation: a field git no longer declares
// is not in the comparison's Modified set, so the rule does not carry it.
// That matches the diff, which does not ignore it either.
//
// Ref: docs/user-guide/diffing.md (managedFieldsManagers example),
// https://github.com/argoproj/argo-cd/discussions/14166,
// https://github.com/argoproj/argo-cd/issues/25112,
// https://github.com/argoproj/argo-cd/issues/25110 (field removed from git).
func TestRespectIgnoreSemantics_HPAReplicasKept_ManagedFieldsManagersRule(t *testing.T) {
	gitV1 := toUnstructured(t, `
apiVersion: apps/v1
kind: Deployment
metadata: {name: web, namespace: default}
spec:
  replicas: 1
  selector: {matchLabels: {app: web}}
  template:
    metadata: {labels: {app: web}}
    spec:
      containers:
      - {name: web, image: nginx:1.26}
`)
	gitV2 := gitV1.DeepCopy()
	containers := podList(t, gitV2, "containers")
	containers[0].(map[string]any)["image"] = "nginx:1.27"
	require.NoError(t, unstructured.SetNestedSlice(gitV2.Object, containers, "spec", "template", "spec", "containers"))

	// Argo CD applies git, then the HPA scales to 4.
	sim := newSSASim(t, builtinTypeConverter(t, deploymentGVK), deploymentGVK)
	sim.apply(gitV1, argoManager)
	scaled := sim.live().DeepCopy()
	unstructured.RemoveNestedField(scaled.Object, "metadata", "managedFields")
	require.NoError(t, unstructured.SetNestedField(scaled.Object, int64(4), "spec", "replicas"))
	sim.update(scaled, "kube-controller-manager")
	live := sim.live()
	ignore := v1alpha1.ResourceIgnoreDifferences{Group: "apps", Kind: "Deployment", ManagedFieldsManagers: []string{"kube-controller-manager"}}

	t.Run("git still declares replicas", func(t *testing.T) {
		applied := applyTarget(t, nil, live, gitV2, ignore)
		replicas, _, err := unstructured.NestedInt64(applied.Object, "spec", "replicas")
		require.NoError(t, err)
		assert.Equal(t, int64(4), replicas, "the HPA's value is carried over git's")

		sim.apply(applied, argoManager)
		out := sim.live()
		replicas, _, err = unstructured.NestedInt64(out.Object, "spec", "replicas")
		require.NoError(t, err)
		assert.Equal(t, int64(4), replicas)
		assert.Equal(t, "nginx:1.27", at(t, out, "spec.template.spec.containers[0].image"))
	})

	t.Run("git no longer declares replicas (#25110)", func(t *testing.T) {
		gitV3 := gitV2.DeepCopy()
		unstructured.RemoveNestedField(gitV3.Object, "spec", "replicas")
		applied := applyTarget(t, nil, live, gitV3, ignore)
		_, found, err := unstructured.NestedInt64(applied.Object, "spec", "replicas")
		require.NoError(t, err)
		assert.False(t, found, "a field absent from git is not Modified, so the rule does not carry it; same as the diff")
	})
}

// Use case 22: all of metadata ignored, as users do for PersistentVolumes an
// operator rewrites. Live labels are carried, but the fields the API server
// populates are not: server-side apply rejects a body with managedFields
// ("metadata.managedFields must be nil") and a stale resourceVersion or uid
// makes the apply fail its preconditions.
//
// Ref: https://github.com/argoproj/argo-cd/discussions/16448.
const wholeMetadataTarget = `
apiVersion: v1
kind: ConfigMap
metadata: {name: cm, namespace: default, labels: {a: git}}
data: {k: git}
`

const wholeMetadataLive = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm
  namespace: default
  labels: {a: live, b: live}
  uid: "1234"
  resourceVersion: "99"
  generation: 3
  creationTimestamp: "2026-01-01T00:00:00Z"
  managedFields:
  - manager: x
    operation: Update
    apiVersion: v1
    fieldsType: FieldsV1
    fieldsV1: {f:metadata: {f:labels: {f:b: {}}}}
data: {k: live}
`

func TestRespectIgnoreSemantics_AllMetadataIgnored_ServerFieldsNotApplied(t *testing.T) {
	target := toUnstructured(t, wholeMetadataTarget)
	live := toUnstructured(t, wholeMetadataLive)
	ignore := v1alpha1.ResourceIgnoreDifferences{Kind: "ConfigMap", JSONPointers: []string{"/metadata"}}

	applied := applyTarget(t, nil, live, target, ignore)

	assert.Equal(t, "cm", applied.GetName())
	assert.Equal(t, "default", applied.GetNamespace())
	assert.Equal(t, map[string]string{"a": "live", "b": "live"}, applied.GetLabels(), "live metadata is carried")
	meta := applied.Object["metadata"].(map[string]any)
	for _, field := range serverMetadataFields {
		assert.NotContains(t, meta, field)
	}
	assert.Equal(t, map[string]any{"k": "git"}, applied.Object["data"], "data is not ignored")

	cmGVK := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	sim := newSSASim(t, builtinTypeConverter(t, cmGVK), cmGVK)
	sim.apply(target, argoManager)
	liveNoMF := live.DeepCopy()
	unstructured.RemoveNestedField(liveNoMF.Object, "metadata", "managedFields")
	sim.update(liveNoMF, otherManager)
	sim.apply(applied, argoManager)
	out := sim.live()
	assert.Equal(t, map[string]string{"a": "live", "b": "live"}, out.GetLabels())
	assert.Equal(t, map[string]any{"k": "git"}, out.Object["data"])
}

// Use case 23: Service nodePort allocated by the server, ignored by pointer or jq.
// Service ports are keyed by (port, protocol) and manifests omit protocol, so
// the live item is matched to git's through the schema default (TCP). Without
// that default the carried nodePort would land in a duplicate port item, which
// both apply modes reject.
//
// Ref: found by code review of this change; protocol's default is declared in
// the API schema (io.k8s.api.core.v1.ServicePort).
const nodePortTarget = `
apiVersion: v1
kind: Service
metadata: {name: svc, namespace: default}
spec:
  type: NodePort
  selector: {app: web}
  ports:
  - {name: http, port: 80, targetPort: 8080}
  - {name: https, port: 443, targetPort: 8443}
`

const nodePortLive = `
apiVersion: v1
kind: Service
metadata: {name: svc, namespace: default}
spec:
  type: NodePort
  selector: {app: web}
  ports:
  - {name: http, port: 80, targetPort: 8080, protocol: TCP, nodePort: 31080}
  - {name: https, port: 443, targetPort: 8443, protocol: TCP, nodePort: 31443}
`

func TestRespectIgnoreSemantics_AllocatedNodePortsKept_GitOmitsProtocol(t *testing.T) {
	target := toUnstructured(t, nodePortTarget)
	live := toUnstructured(t, nodePortLive)
	check := func(t *testing.T, ignore v1alpha1.ResourceIgnoreDifferences) {
		t.Helper()
		applied := applyTarget(t, nil, live, target, ignore)
		require.Equal(t, []string{"http", "https"}, names(at(t, applied, "spec.ports").([]any)), "no duplicate port items")
		assert.Equal(t, int64(31080), at(t, applied, "spec.ports[name=http].nodePort"), "the allocated nodePort is kept")
		assert.Equal(t, int64(31443), at(t, applied, "spec.ports[name=https].nodePort"))
		absent(t, applied, "spec.ports[name=http].protocol")

		sim := newSSASim(t, builtinTypeConverter(t, serviceGVK), serviceGVK)
		sim.apply(target, argoManager)
		sim.update(live, otherManager) // the API server allocates nodePorts
		sim.apply(applied, argoManager)
		synced := sim.live()
		assert.Len(t, at(t, synced, "spec.ports"), 2)
		assert.Equal(t, int64(31080), at(t, synced, "spec.ports[name=http].nodePort"))
	}
	t.Run("pointer", func(t *testing.T) {
		check(t, v1alpha1.ResourceIgnoreDifferences{Kind: "Service", JSONPointers: []string{"/spec/ports/0/nodePort", "/spec/ports/1/nodePort"}})
	})
	t.Run("jq", func(t *testing.T) {
		check(t, v1alpha1.ResourceIgnoreDifferences{Kind: "Service", JQPathExpressions: []string{`.spec.ports[].nodePort`}})
	})
}

// Use case 24: the documented example. An HPA owns spec.replicas, git declares a
// value, and the rule is the jsonPointer from the docs. The live count is
// applied and git's other changes go through, under both apply modes.
//
// Ref: docs/user-guide/diffing.md and docs/user-guide/sync-options.md,
// https://github.com/argoproj/argo-cd/issues/25112,
// https://github.com/argoproj/argo-cd/discussions/14166.
func TestRespectIgnoreSemantics_HPAReplicasKept_JSONPointerRule(t *testing.T) {
	gitV1 := toUnstructured(t, `
apiVersion: apps/v1
kind: Deployment
metadata: {name: web, namespace: default}
spec:
  replicas: 3
  selector: {matchLabels: {app: web}}
  template:
    metadata: {labels: {app: web}}
    spec:
      containers:
      - {name: web, image: nginx:1.26}
`)
	live := gitV1.DeepCopy()
	require.NoError(t, unstructured.SetNestedField(live.Object, int64(5), "spec", "replicas"))
	gitV2 := gitV1.DeepCopy()
	containers := podList(t, gitV2, "containers")
	containers[0].(map[string]any)["image"] = "nginx:1.27"
	require.NoError(t, unstructured.SetNestedSlice(gitV2.Object, containers, "spec", "template", "spec", "containers"))
	ignore := v1alpha1.ResourceIgnoreDifferences{Group: "apps", Kind: "Deployment", JSONPointers: []string{"/spec/replicas"}}

	applied := applyTarget(t, nil, live, gitV2, ignore)
	replicas, _, err := unstructured.NestedInt64(applied.Object, "spec", "replicas")
	require.NoError(t, err)
	assert.Equal(t, int64(5), replicas, "the HPA's count is applied, not git's")
	assert.Equal(t, "nginx:1.27", at(t, applied, "spec.template.spec.containers[0].image"))

	t.Run("ssa", func(t *testing.T) {
		sim := newSSASim(t, builtinTypeConverter(t, deploymentGVK), deploymentGVK)
		sim.apply(gitV1, argoManager)
		sim.update(live, "kube-controller-manager")
		sim.apply(applied, argoManager)
		out := sim.live()
		replicas, _, err := unstructured.NestedInt64(out.Object, "spec", "replicas")
		require.NoError(t, err)
		assert.Equal(t, int64(5), replicas)
		assert.Equal(t, "nginx:1.27", at(t, out, "spec.template.spec.containers[0].image"))
	})
	t.Run("csa", func(t *testing.T) {
		out := csaApply(t, gitV1, live, applied)
		replicas, _, err := unstructured.NestedInt64(out.Object, "spec", "replicas")
		require.NoError(t, err)
		assert.Equal(t, int64(5), replicas)
		assert.Equal(t, "nginx:1.27", at(t, out, "spec.template.spec.containers[0].image"))
	})
}

// Use case 25: status is never carried, even when a rule names it, and a status
// git carries (charts sometimes render one) is dropped by the rule like any
// other git path. For a CRD without a /status subresource, carrying status
// would make the sync manager a co-owner and freeze what the controller wrote.
//
// Ref: https://github.com/argoproj/argo-cd/issues/28201,
// https://github.com/argoproj/argo-cd/issues/29253.
func TestRespectIgnoreSemantics_StatusIgnored_NeverApplied(t *testing.T) {
	target := toUnstructured(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: child, namespace: argocd}
spec:
  project: default
  destination: {server: https://kubernetes.default.svc, namespace: default}
  source: {repoURL: https://example.com/a.git, path: a, targetRevision: main}
status:
  sync: {status: Synced}
`)
	live := toUnstructured(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: child, namespace: argocd}
spec:
  project: default
  destination: {server: https://kubernetes.default.svc, namespace: default}
  source: {repoURL: https://example.com/a.git, path: a, targetRevision: v2}
status:
  sync: {status: OutOfSync}
  operationState: {phase: Running}
`)
	ignore := v1alpha1.ResourceIgnoreDifferences{Group: "argoproj.io", Kind: "Application", JSONPointers: []string{"/status"}}

	applied := applyTarget(t, nil, live, target, ignore)

	absent(t, applied, "status")
	assert.Equal(t, "main", at(t, applied, "spec.source.targetRevision"), "spec is applied from git")
}

// Use case 26: a field inside an item of a keyless list that git does not declare.
// The list is applied as one value, so carrying only the field would put a
// partial item in the cluster (a toleration with a value and no key, which the
// API server rejects). The whole live item is carried.
//
// Ref: found by code review of this change.
func TestRespectIgnoreSemantics_InjectedTolerationValueIgnored_WholeTolerationKept(t *testing.T) {
	target := toUnstructured(t, `
apiVersion: apps/v1
kind: Deployment
metadata: {name: web, namespace: default}
spec:
  selector: {matchLabels: {app: web}}
  template:
    metadata: {labels: {app: web}}
    spec:
      containers:
      - {name: web, image: nginx:1.27}
      tolerations:
      - {key: dedicated, operator: Exists}
`)
	live := toUnstructured(t, `
apiVersion: apps/v1
kind: Deployment
metadata: {name: web, namespace: default}
spec:
  selector: {matchLabels: {app: web}}
  template:
    metadata: {labels: {app: web}}
    spec:
      containers:
      - {name: web, image: nginx:1.26}
      tolerations:
      - {key: dedicated, operator: Exists}
      - {key: node.example.com/spot, operator: Equal, value: "true", effect: NoSchedule}
`)
	ignore := deploymentIgnore(`.spec.template.spec.tolerations[] | select(.key == "node.example.com/spot") | .value`)

	applied := applyTarget(t, nil, live, target, ignore)

	want := []any{
		map[string]any{"key": "dedicated", "operator": "Exists"},
		map[string]any{"key": "node.example.com/spot", "operator": "Equal", "value": "true", "effect": "NoSchedule"},
	}
	assert.Equal(t, want, podList(t, applied, "tolerations"))

	sim := newSSASim(t, builtinTypeConverter(t, deploymentGVK), deploymentGVK)
	sim.apply(target, argoManager)
	sim.update(live, otherManager)
	sim.apply(applied, argoManager)
	out := sim.live()
	assert.Equal(t, want, podList(t, out, "tolerations"))
	assert.Equal(t, "nginx:1.27", at(t, out, "spec.template.spec.containers[0].image"))
}

// Use case 27: the stringData fold follows the API server: a key present in both
// stringData and data takes the stringData value, other data keys stay. Only
// then do the rules run, so a rule on a data key sees the folded manifest.
//
// Ref: https://kubernetes.io/docs/concepts/configuration/secret/ (stringData is merged into data on write);
// https://github.com/argoproj/argo-cd/issues/29644.
func TestRespectIgnoreSemantics_SecretStringDataWinsOverData_LikeTheAPIServer(t *testing.T) {
	target := toUnstructured(t, `
apiVersion: v1
kind: Secret
metadata: {name: s, namespace: default}
type: Opaque
data:
  password: ZnJvbS1kYXRh
  other: b3RoZXI=
stringData:
  password: from-stringData
  username: app
`)
	live := toUnstructured(t, `
apiVersion: v1
kind: Secret
metadata: {name: s, namespace: default}
type: Opaque
data:
  password: bGl2ZQ==
  other: bGl2ZQ==
  username: bGl2ZQ==
`)
	ignore := v1alpha1.ResourceIgnoreDifferences{Kind: "Secret", JSONPointers: []string{"/data/other"}}

	applied := applyTarget(t, nil, live, target, ignore)

	assert.NotContains(t, applied.Object, "stringData")
	data, _, err := unstructured.NestedStringMap(applied.Object, "data")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"password": "ZnJvbS1zdHJpbmdEYXRh", // "from-stringData" wins over data
		"username": "YXBw",                 // "app"
		"other":    "bGl2ZQ==",             // ignored: live value
	}, data)
}

// Use case 28: a resource no rule touches is applied exactly as given,
// as the same object, not a copy.
func TestRespectIgnoreSemantics_NothingIgnored_ManifestAppliedAsIs(t *testing.T) {
	live := toUnstructured(t, injectedInitLive)
	target := toUnstructured(t, injectedInitTarget)
	dc, err := diff.NewDiffConfigBuilder().
		WithDiffSettings([]v1alpha1.ResourceIgnoreDifferences{{Kind: "Service", JSONPointers: []string{"/spec"}}}, nil, true, normalizers.IgnoreNormalizerOpts{}).
		WithGVKParser(builtinGVKParser(t)).
		WithNoCache().
		Build()
	require.NoError(t, err)
	cr := &comparisonResult{
		reconciliationResult: sync.ReconciliationResult{Live: []*unstructured.Unstructured{live}, Target: []*unstructured.Unstructured{target}},
		diffConfig:           dc,
	}

	targets, err := normalizeTargetResources(cr)

	require.NoError(t, err)
	require.Len(t, targets, 1)
	assert.Same(t, target, targets[0], "a resource without ignored paths is applied as given")
}

// ---------------------------------------------------------------------------
// harness: how the cases build their inputs and replay the apply target
// ---------------------------------------------------------------------------

// toUnstructured decodes YAML the way the controller receives objects from the
// API server and repo-server (JSON decode, integers as int64). The existing
// test.YamlToUnstructured helper yields float64 for every number, which does
// not match production.
func toUnstructured(t *testing.T, y string) *unstructured.Unstructured {
	t.Helper()
	j, err := yaml.YAMLToJSON([]byte(y))
	require.NoError(t, err)
	obj := &unstructured.Unstructured{}
	_, _, err = unstructured.UnstructuredJSONScheme.Decode(j, nil, obj)
	require.NoError(t, err)
	return obj
}

type applyOpts struct {
	ignores []v1alpha1.ResourceIgnoreDifferences
	// tracking enables resource-tracking normalization (annotation method),
	// as the controller has it configured in production.
	tracking bool
}

// applyTarget runs normalizeTargetResources for one live/target pair. parser
// is the cluster's GVK parser; nil means a cluster that knows the built-in
// kinds and publishes no CRD schema (see gvkParserFromSchema).
func applyTarget(t *testing.T, parser *managedfields.GvkParser, live, target *unstructured.Unstructured, ignore v1alpha1.ResourceIgnoreDifferences) *unstructured.Unstructured {
	t.Helper()
	return applyTargetWith(t, parser, live, target, applyOpts{ignores: []v1alpha1.ResourceIgnoreDifferences{ignore}})
}

func applyTargetWith(t *testing.T, parser *managedfields.GvkParser, live, target *unstructured.Unstructured, opts applyOpts) *unstructured.Unstructured {
	t.Helper()
	if parser == nil {
		parser = builtinGVKParser(t)
	}
	b := diff.NewDiffConfigBuilder().
		WithDiffSettings(opts.ignores, nil, true, normalizers.IgnoreNormalizerOpts{}).
		WithGVKParser(parser).
		WithNoCache()
	if opts.tracking {
		b = b.WithTracking(common.LabelKeyAppInstance, string(v1alpha1.TrackingMethodAnnotation))
	}
	dc, err := b.Build()
	require.NoError(t, err)
	cr := &comparisonResult{
		reconciliationResult: sync.ReconciliationResult{
			Live:   []*unstructured.Unstructured{live},
			Target: []*unstructured.Unstructured{target},
		},
		diffConfig: dc,
	}
	targets, err := normalizeTargetResources(cr)
	require.NoError(t, err)
	require.Len(t, targets, 1)
	require.NotNil(t, targets[0])
	t.Logf("apply target:\n%s", mustYAML(t, targets[0].Object))
	return targets[0]
}

func mustYAML(t *testing.T, v any) string {
	t.Helper()
	b, err := yaml.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

func podList(t *testing.T, obj *unstructured.Unstructured, field string) []any {
	t.Helper()
	l, _, err := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", field)
	require.NoError(t, err)
	return l
}

// at returns the value at a dotted path in obj, so an assertion reads like the
// field it checks: "spec.template.spec.containers[name=web].image". A list
// item is selected by [index] or by [key=value]. The test fails if the path
// does not exist.
func at(t *testing.T, obj *unstructured.Unstructured, path string) any {
	t.Helper()
	v, ok := lookup(obj.Object, path)
	require.True(t, ok, "%s is absent:\n%s", path, mustYAML(t, obj.Object))
	return v
}

// absent asserts that nothing exists at the dotted path.
func absent(t *testing.T, obj *unstructured.Unstructured, path string) {
	t.Helper()
	v, ok := lookup(obj.Object, path)
	assert.False(t, ok, "%s should be absent, got %v", path, v)
}

func lookup(root any, path string) (any, bool) {
	cur := root
	for _, seg := range splitPath(path) {
		name, selectors := seg, []string(nil)
		if before, after, found := strings.Cut(seg, "["); found {
			name = before
			selectors = strings.Split(strings.TrimSuffix(after, "]"), "][")
		}
		if name != "" {
			m, ok := cur.(map[string]any)
			if !ok {
				return nil, false
			}
			if cur, ok = m[name]; !ok {
				return nil, false
			}
		}
		for _, sel := range selectors {
			l, ok := cur.([]any)
			if !ok {
				return nil, false
			}
			if k, v, isKey := strings.Cut(sel, "="); isKey {
				i := slices.IndexFunc(l, func(x any) bool { m, _ := x.(map[string]any); return fmt.Sprint(m[k]) == v })
				if i < 0 {
					return nil, false
				}
				cur = l[i]
			} else {
				i, err := strconv.Atoi(sel)
				if err != nil || i < 0 || i >= len(l) {
					return nil, false
				}
				cur = l[i]
			}
		}
	}
	return cur, true
}

// splitPath splits a dotted path on the dots outside [...] selectors, so a
// selector value may contain dots ("webhooks[name=inject.example.com]").
func splitPath(path string) []string {
	var segs []string
	depth, start := 0, 0
	for i, c := range path {
		switch c {
		case '[':
			depth++
		case ']':
			depth--
		case '.':
			if depth == 0 {
				segs = append(segs, path[start:i])
				start = i + 1
			}
		}
	}
	return append(segs, path[start:])
}

func names(list []any) []string {
	var out []string
	for _, e := range list {
		m, _ := e.(map[string]any)
		if n, ok := m["name"].(string); ok {
			out = append(out, n)
		} else if k, ok := m["key"].(string); ok {
			out = append(out, k)
		}
	}
	return out
}

// builtinGVKs are the built-in kinds the tests use. The cluster GVK parser only
// needs their names: gitops-engine resolves built-in types from its static
// structured-merge-diff schema, in tests as in production.
var builtinGVKs = []schema.GroupVersionKind{
	deploymentGVK,
	{Version: "v1", Kind: "Secret"},
	{Version: "v1", Kind: "ConfigMap"},
	serviceGVK,
	statefulSetGVK,
	webhookGVK,
	ingressGVK,
}

// staticTypeName returns the name of a built-in kind in the static schema.
func staticTypeName(gvk schema.GroupVersionKind) string {
	group := "core"
	if gvk.Group != "" {
		group = strings.TrimSuffix(gvk.Group, ".k8s.io")
	}
	return fmt.Sprintf("io.k8s.api.%s.%s.%s", group, gvk.Version, gvk.Kind)
}

// gvkParserFromSchema builds the GVK parser the controller gets for a cluster
// that knows the built-in kinds above and publishes the given OpenAPI v2
// documents for its CRDs.
func gvkParserFromSchema(t *testing.T, crdSchemaPaths ...string) *managedfields.GvkParser {
	t.Helper()
	definitions := map[string]any{}
	for _, gvk := range builtinGVKs {
		definitions[staticTypeName(gvk)] = map[string]any{
			"type":                            "object",
			"x-kubernetes-group-version-kind": []any{map[string]any{"group": gvk.Group, "version": gvk.Version, "kind": gvk.Kind}},
		}
	}
	for _, path := range crdSchemaPaths {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		j, err := yaml.YAMLToJSON(data)
		require.NoError(t, err)
		var doc struct {
			Definitions map[string]any `json:"definitions"`
		}
		require.NoError(t, json.Unmarshal(j, &doc))
		maps.Copy(definitions, doc.Definitions)
	}
	raw, err := json.Marshal(map[string]any{
		"swagger":     "2.0",
		"info":        map[string]any{"title": "test", "version": "v0"},
		"paths":       map[string]any{},
		"definitions": definitions,
	})
	require.NoError(t, err)
	doc, err := openapi_v2.ParseDocument(raw)
	require.NoError(t, err)
	models, err := proto.NewOpenAPIData(doc)
	require.NoError(t, err)
	parser, err := managedfields.NewGVKParser(models, false)
	require.NoError(t, err)
	return parser
}

// builtinGVKParser models a cluster that publishes no schema for any CRD.
func builtinGVKParser(t *testing.T) *managedfields.GvkParser {
	t.Helper()
	return gvkParserFromSchema(t)
}

// staticTypeConverter is an SSA type converter for a built-in kind, backed by
// the static structured-merge-diff schema.
type staticTypeConverter struct {
	pt typed.ParseableType
}

func builtinTypeConverter(t *testing.T, gvk schema.GroupVersionKind) managedfields.TypeConverter {
	t.Helper()
	pt := enginescheme.StaticParser().Type(staticTypeName(gvk))
	require.True(t, pt.IsValid(), "no static schema for %s", gvk)
	return staticTypeConverter{pt: pt}
}

func (c staticTypeConverter) ObjectToTyped(obj runtime.Object, opts ...typed.ValidationOptions) (*typed.TypedValue, error) {
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return c.pt.FromUnstructured(content, opts...)
}

func (c staticTypeConverter) TypedToObject(tv *typed.TypedValue) (runtime.Object, error) {
	content, ok := tv.AsValue().Unstructured().(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected a map, got %T", tv.AsValue().Unstructured())
	}
	return &unstructured.Unstructured{Object: content}, nil
}

// crdTypeConverter builds an SSA type converter from the same OpenAPI v2
// fixture the controller is given as the cluster schema for a CRD.
func crdTypeConverter(t *testing.T, path string) managedfields.TypeConverter {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	j, err := yaml.YAMLToJSON(data)
	require.NoError(t, err)
	var sw spec.Swagger
	require.NoError(t, json.Unmarshal(j, &sw))
	defs := map[string]*spec.Schema{}
	for k, v := range sw.Definitions {
		defs[k] = &v
	}
	tc, err := managedfields.NewTypeConverter(defs, false)
	require.NoError(t, err)
	return tc
}

const (
	// Rollout schema carrying the legacy x-kubernetes-patch-merge-key
	// extensions on the container lists. This is NOT what the real Rollouts
	// CRD publishes; the realistic schema below is.
	rolloutSchemaWithPatchMeta = "testdata/schemas/rollout-podspec-openapi-v2.yaml"
	// Same schema without those extensions, as the real CRD publishes it.
	rolloutSchemaRealistic = "testdata/schemas/rollout-crd-realistic-openapi-v2.yaml"
	applicationSchema      = "testdata/schemas/application-openapi-v2.yaml"
)

// ssaSim drives the real server-side-apply field manager against an
// in-memory live object.
type ssaSim struct {
	t  *testing.T
	fm managedfieldstest.TestFieldManager
}

func newSSASim(t *testing.T, tc managedfields.TypeConverter, gvk schema.GroupVersionKind) *ssaSim {
	t.Helper()
	return &ssaSim{t: t, fm: managedfieldstest.NewTestFieldManager(tc, gvk)}
}

// apply is a forced server-side apply, which is what Argo CD issues when
// ServerSideApply=true (it always sets force).
func (s *ssaSim) apply(obj *unstructured.Unstructured, manager string) {
	s.t.Helper()
	require.NoError(s.t, s.fm.Apply(obj.DeepCopy(), manager, true))
}

// update is a plain PUT/PATCH by a controller, webhook or human: the manager
// takes ownership of every field it changed.
func (s *ssaSim) update(obj *unstructured.Unstructured, manager string) {
	s.t.Helper()
	require.NoError(s.t, s.fm.Update(obj.DeepCopy(), manager))
}

func (s *ssaSim) live() *unstructured.Unstructured {
	s.t.Helper()
	u, ok := s.fm.Live().(*unstructured.Unstructured)
	require.True(s.t, ok)
	s.t.Logf("live after SSA:\n%s", mustYAML(s.t, u.Object))
	return u
}

// csaApply reproduces kubectl's client-side apply: a three-way strategic
// merge patch between last-applied, the new config and live, applied to live.
func csaApply(t *testing.T, lastApplied, live, target *unstructured.Unstructured) *unstructured.Unstructured {
	t.Helper()
	gvk := target.GroupVersionKind()
	versioned, err := scheme.Scheme.New(gvk)
	require.NoError(t, err)
	meta, err := strategicpatch.NewPatchMetaFromStruct(versioned)
	require.NoError(t, err)
	orig, err := lastApplied.MarshalJSON()
	require.NoError(t, err)
	mod, err := target.MarshalJSON()
	require.NoError(t, err)
	cur, err := live.MarshalJSON()
	require.NoError(t, err)
	patch, err := strategicpatch.CreateThreeWayMergePatch(orig, mod, cur, meta, true)
	require.NoError(t, err)
	t.Logf("CSA patch: %s", patch)
	patchedJSON, err := strategicpatch.StrategicMergePatch(cur, patch, versioned)
	require.NoError(t, err)
	out := &unstructured.Unstructured{}
	_, _, err = unstructured.UnstructuredJSONScheme.Decode(patchedJSON, nil, out)
	require.NoError(t, err)
	t.Logf("live after CSA:\n%s", mustYAML(t, out.Object))
	return out
}

var (
	deploymentGVK  = schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	ingressGVK     = schema.GroupVersionKind{Group: "networking.k8s.io", Version: "v1", Kind: "Ingress"}
	rolloutGVK     = schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Rollout"}
	serviceGVK     = schema.GroupVersionKind{Version: "v1", Kind: "Service"}
	statefulSetGVK = schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}
	webhookGVK     = schema.GroupVersionKind{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "MutatingWebhookConfiguration"}
)

func deploymentIgnore(exprs ...string) v1alpha1.ResourceIgnoreDifferences {
	return v1alpha1.ResourceIgnoreDifferences{Group: "apps", Kind: "Deployment", JQPathExpressions: exprs}
}

func rolloutIgnore(exprs ...string) v1alpha1.ResourceIgnoreDifferences {
	return v1alpha1.ResourceIgnoreDifferences{Group: "argoproj.io", Kind: "Rollout", JQPathExpressions: exprs}
}
