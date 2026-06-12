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
  if obj.status.domainStatus ~= nil then
    for i, domainStatus in ipairs(obj.status.domainStatus) do
      if domainStatus.status == "FailedNotVisible" then
        hs.status = "Degraded"
        hs.message = "At least one certificate has failed to be provisioned"
        return hs
      end
    end
  end
end

if obj.status ~= nil and obj.status.certificateStatus == "Active" then
  hs.status = "Healthy"
  hs.message = "All certificates are active"
  return hs
end

hs.status = "Progressing"
hs.message = "At least one certificate is still being provisioned"
return hs
