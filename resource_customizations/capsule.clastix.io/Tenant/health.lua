-- Tenant is a cluster-scoped resource managed by Capsule that represents a group
-- of namespaces owned by one or more owners (users, service accounts, groups).
--
-- Documentation:
--   https://projectcapsule.dev/
--   https://github.com/projectcapsule/capsule
--
-- Condition types:
--   Ready      (True)  - Tenant has been successfully reconciled
--   Ready      (False) - Tenant reconciliation failed
--   Cordoned   (True)  - Tenant is cordoned; no new workloads can be scheduled
--
-- ArgoCD health mapping:
--   Cordoned=True  => Suspended  (tenant intentionally paused)
--   Ready=True     => Healthy
--   Ready=False    => Degraded
--   No status yet  => Progressing
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
  if condition.type == "Cordoned" and condition.status == "True" then
    hs.status = "Suspended"
    hs.message = condition.message
    return hs
  end
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
