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
if (obj.status.errMsg == nil or obj.status.errMsg == "") and obj.status.value ~= nil then
    hs.status = "Healthy"
    hs.message = "KeptnMetric is healthy"
    return hs
end
if obj.status.errMsg ~= nil and obj.status.errMsg ~= "" then
    hs.status = "Degraded"
    hs.message = "KeptnMetric is degraded"
    return hs
end
hs.status = "Progressing"
hs.message = "KeptnMetric is progressing"
return hs
