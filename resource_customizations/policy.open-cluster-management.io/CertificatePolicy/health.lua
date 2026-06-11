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
if obj.status == nil or obj.status.compliant == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for the status to be reported"
  return hs
end
if obj.status.compliant == "Compliant" then
  hs.status = "Healthy"
  hs.message = "All certificates found comply with the policy"
  return hs
else
  hs.status = "Degraded"
  hs.message = "At least once certificate does not comply with the policy"
  return hs
end
