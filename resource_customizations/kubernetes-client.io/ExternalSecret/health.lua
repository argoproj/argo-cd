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

local health_status = {}
if obj.status ~= nil then
  if obj.status.status == "SUCCESS" then
    health_status.status = "Healthy"
    health_status.message = "Fetched ExternalSecret."
  elseif obj.status.status:find('^ERROR') ~= nil then
    health_status.status = "Degraded"
    health_status.message = obj.status.status:gsub("ERROR, ", "")
  else
    health_status.status = "Progressing"
    health_status.message = "Waiting for ExternalSecret."
  end
  return health_status
end
health_status.status = "Progressing"
health_status.message = "Waiting for ExternalSecret."
return health_status