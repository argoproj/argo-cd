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
  if obj.status.phase == "Running" then
    health_status.status = "Healthy"
    health_status.message = "Jaeger is Running"
    return health_status
  end
  if obj.status.phase == "Failed" then
    health_status.status = "Degraded"
    health_status.message = "Jaeger Failed For Some Reason"
    return health_status
  end
end
health_status.status = "Progressing"
health_status.message = "Waiting for Jaeger"
return health_status