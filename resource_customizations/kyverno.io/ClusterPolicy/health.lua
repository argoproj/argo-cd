-- Surface deletion progress while the resource is terminating. You can customize this
-- block, e.g. map known finalizers in obj.metadata.finalizers to clearer messages.
if obj.metadata ~= nil and obj.metadata.deletionTimestamp ~= nil then
  local deletionHs = {}
  deletionHs.status = "Progressing"
  deletionHs.message = "Pending deletion"
  if obj.metadata.finalizers ~= nil and #obj.metadata.finalizers > 0 then
    deletionHs.message = "Pending deletion; blocked by finalizers: " .. table.concat(obj.metadata.finalizers, ", ")
  end
  return deletionHs
end

-- ClusterPolicy is a cluster-scoped resource that defines policy validation,
-- mutation, and generation behaviors for matching Kubernetes resources.
--
-- Documentation:
--   Policy types overview: https://kyverno.io/docs/policy-types/cluster-policy/
--
-- Condition types and reasons are defined in:
--   https://github.com/kyverno/kyverno/blob/main/api/kyverno/v1/clusterpolicy_types.go
--
-- ClusterPolicy exposes one active condition type:
--   Ready (True)  - Policy is fully loaded and validated, rules are active
--   Ready (False) - Policy failed to load (syntax error, missing resources, etc.)
--
-- ArgoCD health mapping:
--   Ready=True   => Healthy
--   Ready=False  => Degraded
--   No status yet => Progressing
local hs = {}

if obj.status ~= nil and obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Ready" then
      if condition.status == "True" then
        hs.status = "Healthy"
        hs.message = condition.message
      else
        hs.status = "Degraded"
        hs.message = condition.message
      end
      return hs
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for ClusterPolicy to become ready"
return hs
