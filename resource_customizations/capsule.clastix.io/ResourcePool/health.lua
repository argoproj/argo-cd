-- ResourcePool is a cluster-scoped resource that defines a shared pool of
-- resource quotas that can be claimed by tenant namespaces.
--
-- Documentation:
--   https://projectcapsule.dev/
--
-- ArgoCD health mapping:
--   exhaustions set => Degraded  (one or more resources are exhausted)
--   Ready=True      => Healthy
--   Ready=False     => Degraded
--   No status       => Progressing
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

if obj.status.exhaustions ~= nil then
  local exhausted = {}
  for resource, _ in pairs(obj.status.exhaustions) do
    table.insert(exhausted, resource)
  end
  table.sort(exhausted)
  if #exhausted > 0 then
    hs.status = "Degraded"
    hs.message = "Pool exhausted for: " .. table.concat(exhausted, ", ")
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
