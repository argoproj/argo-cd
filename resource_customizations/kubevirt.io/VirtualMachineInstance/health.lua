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
    if hs.message == "Failed" then
      hs.status = "Degraded"
      return hs
    elseif hs.message == "Pending" or hs.message == "Scheduling" or hs.message == "Scheduled" then
      return hs
    elseif hs.message == "Succeeded" then
      hs.status = "Suspended"
      return hs
    elseif hs.message == "Unknown" then
      hs.status = "Unknown"
      return hs
    end
  end
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "Ready" then
        if condition.status == "True" then
          hs.status = "Healthy"
          hs.message = "Running"
        else
          hs.status = "Degraded"
          hs.message = condition.message
        end
      elseif condition.type == "Paused" and condition.status == "True" then
        hs.status = "Suspended"
        hs.message = condition.message
        return hs
      end
    end
  end
end
return hs
