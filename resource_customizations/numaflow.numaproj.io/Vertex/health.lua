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
local healthy = {}

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.status == "False" then
        healthy.status = "False"
        healthy.message = condition.message
      end
    end
  end

  if obj.metadata.generation == obj.status.observedGeneration then
    if (healthy ~= {} and healthy.status == "False") or obj.status.phase == "Failed" then
      hs.status = "Degraded"
      if obj.status.phase == "Failed" then
        hs.message = obj.status.message
      else
        hs.message = healthy.message
      end
      return hs
    elseif obj.status.phase == "Paused" then
      hs.status = "Healthy"
      hs.message = "Vertex is paused"
      return hs
    elseif obj.status.phase == "Running" then
      hs.status = "Healthy"
      hs.message = "Vertex is healthy"
      return hs
    end
  end
end
    
hs.status = "Progressing"
hs.message = "Waiting for Vertex status"
return hs