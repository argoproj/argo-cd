-- NamespacedGeneratingPolicy is a namespace-scoped resource that automatically
-- generates Kubernetes resources based on CEL-based match rules and trigger
-- conditions, scoped to a specific namespace.
--
-- Documentation:
--   Policy types overview: https://kyverno.io/docs/policy-types/generating-policy/
--
-- Condition types and reasons are defined in:
--   https://github.com/kyverno/kyverno/tree/main/config/crds/policies.kyverno.io/policies.kyverno.io_namespacedgeneratingpolicies.yaml
--
-- NamespacedGeneratingPolicy exposes a conditionStatus with a ready boolean and
-- standard Kubernetes conditions. No fixed condition type names are enforced.
--
-- ArgoCD health mapping:
--   conditionStatus.ready=true   => Healthy  (message from first True condition)
--   conditionStatus.ready=false  => Degraded (message from first False condition)
--   No status yet                => Progressing
local hs = {}

if obj.status ~= nil and obj.status.conditionStatus ~= nil then
  local cs = obj.status.conditionStatus

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
hs.message = "Waiting for NamespacedGeneratingPolicy status"
return hs
