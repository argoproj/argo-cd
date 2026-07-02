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

local hs = { status="Progressing", message="No status available"}
if obj.status ~= nil then
  if obj.status.phase ~= nil then
    hs.message = obj.status.phase
    if hs.message == "Succeeded" then
      hs.status = "Healthy"
      return hs
    elseif hs.message == "Failed" or hs.message == "Unknown" then
      hs.status = "Degraded"
    elseif hs.message == "Paused" then
      hs.status = "Suspended"
      return hs
    end
  end
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "Running" and condition.status == "False" and condition.reason == "Error" then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
      end
    end
  end
end
return hs
