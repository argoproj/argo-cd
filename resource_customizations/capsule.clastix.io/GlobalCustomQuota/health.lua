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

-- GlobalCustomQuota is a cluster-scoped resource that defines a custom resource
-- quota across all namespaces belonging to a Capsule Tenant.
--
-- Documentation:
--   API types:       https://github.com/projectcapsule/capsule/blob/main/api/v1beta2/globalcustomquota_types.go
--   Controller:      https://github.com/projectcapsule/capsule/blob/main/internal/controllers/customquotas/global_custom_quota_controller.go
--   Condition types: https://github.com/projectcapsule/capsule/blob/main/pkg/api/meta/conditions.go
--
-- ArgoCD health mapping:
--   Ready=True  => Healthy
--   Ready=False => Degraded
--   No status   => Progressing
local hs = {}
if obj.status == nil or obj.status.conditions == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for status"
  return hs
end

if obj.metadata ~= nil and obj.metadata.generation ~= nil and obj.status.observedGeneration ~= nil
    and obj.status.observedGeneration ~= obj.metadata.generation then
  hs.status = "Progressing"
  hs.message = "Waiting for reconciliation (generation mismatch)"
  return hs
end

for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "Ready" and condition.status == "False" then
    hs.status = "Degraded"
    hs.message = condition.message
    return hs
  end
  if condition.type == "Ready" and condition.status == "True" then
    hs.status = "Healthy"
    hs.message = condition.message
    return hs
  end
end

hs.status = "Progressing"
hs.message = "Waiting for Ready condition"
return hs
