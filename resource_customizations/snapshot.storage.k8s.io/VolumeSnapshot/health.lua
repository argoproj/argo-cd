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

if obj.status ~= nil and obj.status.readyToUse then
  hs.status = "Healthy"
  hs.message = "Ready to use"
  return hs
end

if obj.status ~= nil and obj.status.error ~= nil then
  hs.status = "Degraded"
  hs.message = obj.status.error.message
  return hs
end

hs.status = "Progressing"
hs.message = "Waiting for status"

return hs
