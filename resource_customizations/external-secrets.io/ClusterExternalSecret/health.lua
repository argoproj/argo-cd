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
    -- For ClusterExternalSecret, new statuses are appended to the end of the list
    local lastStatus = obj.status.conditions[#obj.status.conditions]
    if lastStatus.type == "Ready" and lastStatus.status == "True" then
      hs.status = "Healthy"
      hs.message = lastStatus.message
      return hs
    end
    if lastStatus.type == "PartiallyReady" and lastStatus.status == "True" then
      hs.status = "Degraded"
      hs.message = lastStatus.message
      return hs
    end
    if lastStatus.type == "NotReady" and lastStatus.status == "True" then
      hs.status = "Degraded"
      hs.message = lastStatus.message
      return hs
    end
  end
end
hs.status = "Progressing"
hs.message = "Waiting for ClusterExternalSecret"
return hs
