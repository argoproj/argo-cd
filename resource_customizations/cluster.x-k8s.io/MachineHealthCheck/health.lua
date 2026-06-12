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

hs.status = "Progressing"
hs.message = ""

if obj.status ~= nil and obj.status.currentHealthy ~= nil then
    if obj.status.expectedMachines == obj.status.currentHealthy then
        hs.status = "Healthy"
    else
        hs.status = "Degraded"
    end
end

return hs
