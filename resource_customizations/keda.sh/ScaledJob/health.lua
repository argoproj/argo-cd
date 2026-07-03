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
local healthy = false
local degraded = false
local suspended = false
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.status == "False" and condition.type == "Ready" then
        hs.message = condition.message
        degraded = true
      end
      if condition.status == "True" and condition.type == "Ready" then
        hs.message = condition.message
        healthy = true
      end
      if condition.status == "True" and condition.type == "Paused" then
        hs.message = condition.message
        suspended = true
      end
      if condition.status == "Unknown" and condition.type == "Ready" then
        hs.message = condition.message
        degraded = true
      end
    end
  end
end
if degraded == true then
  hs.status = "Degraded"
  return hs
elseif healthy == true and suspended == false then
  hs.status = "Healthy"
  return hs 
elseif healthy == true and suspended == true then
  hs.status = "Suspended"
  return hs
end
hs.status = "Progressing"
hs.message = "Waiting for ScaledJob"
return hs
