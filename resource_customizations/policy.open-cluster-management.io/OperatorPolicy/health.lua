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

hs = {}
if obj.status == nil or obj.status.conditions == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for the status to be reported"
  return hs
end
if obj.status.observedGeneration ~= nil and obj.status.observedGeneration ~= obj.metadata.generation then
  hs.status = "Progressing"
  hs.message = "Waiting for the status to be updated"
  return hs  
end
for i, condition in ipairs(obj.status.conditions) do
  if condition.type == "Compliant" then
    hs.message = condition.message
    if condition.status == "True" then
      hs.status = "Healthy"
      return hs
    else
      hs.status = "Degraded"
      return hs
    end
  end
end
hs.status = "Progressing"
hs.message = "Waiting for the compliance condition"
return hs
