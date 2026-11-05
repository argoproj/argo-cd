-- DeletingPolicy is a cluster-scoped resource that defines scheduled deletion
-- of Kubernetes resources matching a set of CEL-based constraints.
--
-- Documentation:
--   Policy types overview: https://kyverno.io/docs/policy-types/deleting-policy/
--
-- Condition types and reasons are defined in:
--   https://github.com/kyverno/kyverno/tree/main/config/crds/policies.kyverno.io/policies.kyverno.io_deletingpolicies.yaml
--
-- Kyverno's deleting controller (<= 1.18) only writes status.lastExecutionTime after a
-- scheduled run and never sets conditionStatus.ready, so a missing ready field must not
-- be treated as unhealthy.
--
-- ArgoCD health mapping:
--   conditionStatus.ready=false               => Degraded (message from first False condition)
--   conditionStatus.ready unset, False cond.  => Degraded (message from that condition)
--   anything else (incl. no status yet)       => Healthy
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

  local failed = nil
  if cs.conditions ~= nil then
    for _, condition in ipairs(cs.conditions) do
      if condition.status == "False" then
        failed = condition
        break
      end
    end
  end

  if cs.ready == false or (cs.ready == nil and failed ~= nil) then
    hs.status = "Degraded"
    if failed ~= nil then
      hs.message = failed.type .. ": " .. failed.message
    else
      hs.message = (cs.message ~= nil and cs.message ~= "") and cs.message or "Policy is not ready"
    end
    return hs
  end

  hs.status = "Healthy"
  if cs.conditions ~= nil then
    for _, condition in ipairs(cs.conditions) do
      if condition.status == "True" then
        hs.message = condition.message
        break
      end
    end
  end
  if hs.message == nil or hs.message == "" then
    hs.message = cs.message
  end
  if hs.message == nil or hs.message == "" then
    if obj.status.lastExecutionTime ~= nil then
      hs.message = "Last executed at " .. obj.status.lastExecutionTime
    else
      hs.message = "Policy is ready"
    end
  end
  return hs
end

hs.status = "Healthy"
hs.message = "Policy has not been executed yet"
return hs
