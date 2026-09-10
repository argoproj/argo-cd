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

local hs = {}
hs.status = "Progressing"
hs.message = "Waiting for the stack to be reconciled"

if obj.status == nil or obj.status.observedGeneration ~= obj.metadata.generation then
  hs.message = "Waiting for the operator to observe the latest spec change"
  return hs
end

if obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Stalled" and condition.status == "True" then
      hs.status = "Degraded"
      hs.message = condition.message
      return hs
    end
    if condition.type == "Reconciling" and condition.status == "True" then
      hs.status = "Progressing"
      hs.message = condition.message
      return hs
    end
    if condition.type == "Ready" and condition.status == "True" then
      hs.status = "Healthy"
      hs.message = condition.message
      return hs
    end
  end
end

return hs
