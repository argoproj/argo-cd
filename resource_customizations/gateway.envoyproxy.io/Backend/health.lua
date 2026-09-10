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

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for _, condition in ipairs(obj.status.conditions) do
      if condition.type == "Accepted" then
        if condition.observedGeneration ~= nil and condition.observedGeneration ~= obj.metadata.generation then
          hs.status = "Progressing"
          hs.message = "Waiting for Backend status to be updated"
          return hs
        end
        if condition.status == "True" then
          hs.status = "Healthy"
          hs.message = condition.message
          return hs
        else
          hs.status = "Degraded"
          hs.message = condition.message
          return hs
        end
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for Backend status"
return hs
