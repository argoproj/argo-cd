-- DeletingPolicy is a cluster-scoped resource that defines scheduled deletion
-- of Kubernetes resources matching a set of CEL-based constraints.
--
-- Documentation:
--   Policy types overview: https://kyverno.io/docs/policy-types/deleting-policy/
--
-- Condition types and reasons are defined in:
--   https://github.com/kyverno/kyverno/tree/main/config/crds/policies.kyverno.io/policies.kyverno.io_deletingpolicies.yaml
--
-- DeletingPolicy exposes a conditionStatus with a ready boolean and standard
-- Kubernetes conditions. No fixed condition type names are enforced by the CRD.
--
-- ArgoCD health mapping:
--   conditionStatus.ready=true   => Healthy  (message from first True condition)
--   conditionStatus.ready=false  => Degraded (message from first False condition)
--   No status yet                => Progressing
local hs = {}

if obj.status ~= nil and obj.status.conditionStatus ~= nil then
  local cs = obj.status.conditionStatus

  if obj.metadata.generation ~= nil and cs.conditions ~= nil then
    for _, condition in ipairs(cs.conditions) do
      if condition.observedGeneration ~= nil and condition.observedGeneration < obj.metadata.generation then
        hs.status = "Progressing"
        hs.message = "Waiting for DeletingPolicy status to reflect latest generation"
        return hs
      end
    end
  end

  if cs.ready == true then
    hs.status = "Healthy"
    if cs.conditions ~= nil then
      for _, condition in ipairs(cs.conditions) do
        if condition.status == "True" then
          hs.message = condition.message
          break
        end
      end
    end
    if hs.message == nil then
      hs.message = (cs.message ~= nil and cs.message ~= "") and cs.message or "Policy is ready"
    end
    return hs
  end

  hs.status = "Degraded"
  if cs.conditions ~= nil then
    for _, condition in ipairs(cs.conditions) do
      if condition.status == "False" then
        hs.message = condition.type .. ": " .. condition.message
        return hs
      end
    end
  end
  hs.message = (cs.message ~= nil and cs.message ~= "") and cs.message or "Policy is not ready"
  return hs
end

hs.status = "Progressing"
hs.message = "Waiting for DeletingPolicy status"
return hs
