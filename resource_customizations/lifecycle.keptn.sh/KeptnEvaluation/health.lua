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
if obj.status.overallStatus == "Succeeded" then
    hs.status = "Healthy"
    hs.message = "KeptnEvaluation is healthy"
    return hs
end
if obj.status.overallStatus == "Failed" then
    hs.status = "Degraded"
    hs.message = "KeptnEvaluation is degraded"
    return hs
end
hs.status = "Progressing"
hs.message = "KeptnEvaluation is progressing"
return hs