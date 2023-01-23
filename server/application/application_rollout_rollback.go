package application

import (
	"encoding/json"
	"fmt"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/resource"
	jsonpatch "github.com/evanphx/json-patch"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
)

func (s *Server) RollbackApplicationRollout(ctx context.Context, q *application.ApplicationRolloutRollbackRequest) (*application.ApplicationRolloutRollbackResponse, error) {
	a, err := s.appLister.Get(q.GetName())
	if err != nil {
		return nil, fmt.Errorf("error getting application by name: %w", err)
	}

	config, err := s.getApplicationClusterConfig(ctx, a)
	if err != nil {
		return nil, fmt.Errorf("error getting application(%s) cluster config: %w", a.GetName(), err)
	}

	liveRolloutObj, err := s.kubectl.GetResource(ctx, config, getRolloutGVK(), q.GetRolloutName(), q.GetRolloutNamespace())
	if err != nil {
		return nil, fmt.Errorf("error getting live state of rollout(%s): %w", q.GetRolloutName(), err)
	}

	currentRolloutRevision := resource.GetRevision(liveRolloutObj)
	targetRolloutRevision := q.GetRolloutRevision()
	newRolloutRevision := currentRolloutRevision + 1
	if targetRolloutRevision == currentRolloutRevision {
		return nil, fmt.Errorf("revisions are equal, rollback is redundant: %w", err)
	}
	if targetRolloutRevision > currentRolloutRevision {
		return nil, fmt.Errorf("revision greater than latest(%d): %w", currentRolloutRevision, err)
	}

	rs, err := s.getReplicaSetForRolloutRollack(ctx, config, q, a)
	if err != nil {
		return nil, err
	}

	newRolloutObj, err := s.getNewRolloutObjForRollbackPatch(liveRolloutObj, rs)
	if err != nil {
		return nil, err
	}

	_, err = s.patchResourceOnCluster(ctx, config, newRolloutObj, liveRolloutObj)
	if err != nil {
		return nil, err
	}

	return &application.ApplicationRolloutRollbackResponse{
		Rollout:     q.RolloutName,
		NewRevision: &newRolloutRevision,
	}, nil
}

func (s *Server) getRsOfSpecificRevision(ctx context.Context, config *rest.Config, rollout *v1alpha1.ResourceNode, replicasNodes []v1alpha1.ResourceNode, toRevision int64) (*unstructured.Unstructured, error) {
	var (
		latestReplicaSet   *unstructured.Unstructured
		latestRevision     = int64(-1)
		previousReplicaSet *unstructured.Unstructured
		previousRevision   = int64(-1)
	)
	for _, rsNode := range replicasNodes {
		rsliveObj, err := s.kubectl.GetResource(ctx, config, rsNode.GroupKindVersion(), rsNode.Name, rsNode.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error getting resource: %w", err)
		}

		if v := resource.GetRevision(rsliveObj); err == nil {
			if toRevision == 0 {
				if latestRevision < v {
					// newest one we've seen so far
					previousRevision = latestRevision
					previousReplicaSet = latestReplicaSet
					latestRevision = v
					latestReplicaSet = rsliveObj
				} else if previousRevision < v {
					// second newest one we've seen so far
					previousRevision = v
					previousReplicaSet = rsliveObj
				}
			} else if toRevision == v {
				return rsliveObj, nil
			}
		}
	}

	if toRevision > 0 {
		return nil, fmt.Errorf("unable to find specified revision %v in history", toRevision)
	}

	if previousReplicaSet == nil {
		return nil, fmt.Errorf("no revision found for rollout %q", rollout.Name)
	}

	return previousReplicaSet, nil
}

func (s *Server) getNewRolloutObjForRollbackPatch(liveRolloutObj *unstructured.Unstructured, rs *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	rsSpecTemplate, found, err := unstructured.NestedFieldCopy(rs.Object, "spec", "template")
	if !found {
		return nil, fmt.Errorf("failed to found replicaset %s - spec/template: %w", rs.GetName(), err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to copy replicaset %s spec/template: %w", rs.GetName(), err)
	}
	rsSpecTemplateU := unstructured.Unstructured{
		Object: rsSpecTemplate.(map[string]interface{}),
	}
	unstructured.RemoveNestedField(rsSpecTemplateU.Object, "metadata", "labels", "rollouts-pod-template-hash")
	newRolloutObj := liveRolloutObj.DeepCopy()
	err = unstructured.SetNestedField(newRolloutObj.Object, rsSpecTemplateU.Object, "spec", "template")
	if err != nil {
		return nil, fmt.Errorf("failed to set spec/template of rollout %s: %w", liveRolloutObj.GetName(), err)
	}

	return newRolloutObj, nil
}

func (s *Server) getReplicaSetForRolloutRollack(ctx context.Context, config *rest.Config, q *application.ApplicationRolloutRollbackRequest, a *v1alpha1.Application) (*unstructured.Unstructured, error) {
	tree, err := s.GetAppResources(ctx, a)
	if err != nil {
		return nil, fmt.Errorf("error getting app resources: %w", err)
	}

	rolloutGVK := getRolloutGVK()

	foundRolloutNode := tree.FindNode(rolloutGVK.Group, rolloutGVK.Kind, q.GetRolloutNamespace(), q.GetRolloutName())
	if foundRolloutNode == nil || foundRolloutNode.ResourceRef.UID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "%s %s %s not found as part of application %s", rolloutGVK.Kind, rolloutGVK.Group, q.GetRolloutName(), q.GetName())
	}

	childReplicaSets := foundRolloutNode.GetAllChildNodes(tree, "ReplicaSet")

	if len(childReplicaSets) == 0 {
		return nil, fmt.Errorf("no related replicasets found for rollout %s: %w", q.GetRolloutName(), err)
	}

	rs, err := s.getRsOfSpecificRevision(ctx, config, foundRolloutNode, childReplicaSets, q.GetRolloutRevision())
	if rs == nil {
		return nil, fmt.Errorf("no related replicaset of revision %d was found for rollout %s: %w", q.GetRolloutRevision(), q.GetRolloutName(), err)
	}
	if err != nil {
		return nil, err
	}

	return rs, nil
}

func getRolloutGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "Rollout",
	}
}

// lagic taken from ./application.go - RunResourceAction
func (s *Server) patchResourceOnCluster(ctx context.Context, config *rest.Config, newObj *unstructured.Unstructured, liveObj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	newObjBytes, err := json.Marshal(newObj)
	if err != nil {
		return nil, fmt.Errorf("error marshaling new object: %w", err)
	}

	liveObjBytes, err := json.Marshal(liveObj)
	if err != nil {
		return nil, fmt.Errorf("error marshaling live object: %w", err)
	}

	diffBytes, err := jsonpatch.CreateMergePatch(liveObjBytes, newObjBytes)
	if err != nil {
		return nil, fmt.Errorf("error calculating merge patch: %w", err)
	}
	if string(diffBytes) == "{}" {
		return nil, nil
	}

	// The following logic detects if the resource action makes a modification to status and/or spec.
	// If status was modified, we attempt to patch the status using status subresource, in case the
	// CRD is configured using the status subresource feature. See:
	// https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#status-subresource
	// If status subresource is in use, the patch has to be split into two:
	// * one to update spec (and other non-status fields)
	// * the other to update only status.
	nonStatusPatch, statusPatch, err := splitStatusPatch(diffBytes)
	if err != nil {
		return nil, fmt.Errorf("error splitting status patch: %w", err)
	}
	if statusPatch != nil {
		_, err = s.kubectl.PatchResource(ctx, config, newObj.GroupVersionKind(), newObj.GetName(), newObj.GetNamespace(), types.MergePatchType, diffBytes, "status")
		if err != nil {
			if !apierr.IsNotFound(err) {
				return nil, fmt.Errorf("error patching resource: %w", err)
			}
			// K8s API server returns 404 NotFound when the CRD does not support the status subresource
			// if we get here, the CRD does not use the status subresource. We will fall back to a normal patch
		} else {
			// If we get here, the CRD does use the status subresource, so we must patch status and
			// spec separately. update the diffBytes to the spec-only patch and fall through.
			diffBytes = nonStatusPatch
		}
	}
	if diffBytes != nil {
		result, err := s.kubectl.PatchResource(ctx, config, newObj.GroupVersionKind(), newObj.GetName(), newObj.GetNamespace(), types.MergePatchType, diffBytes)
		if err != nil {
			return nil, fmt.Errorf("error patching resource: %w", err)
		}

		return result, nil
	}
	return nil, nil
}
