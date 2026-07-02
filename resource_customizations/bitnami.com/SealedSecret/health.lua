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

local health_status={}
if obj.status ~= nil then
    if obj.status.conditions ~= nil then
        for i, condition in ipairs(obj.status.conditions) do
            if condition.type == "Synced" and condition.status == "False" then
                health_status.status = "Degraded"
                health_status.message = condition.message
                return health_status
            end
            if condition.type == "Synced" and condition.status == "True" then
                health_status.status = "Healthy"
                health_status.message = condition.message
                return health_status
            end
        end
    end
end
health_status.status = "Progressing"
health_status.message = "Waiting for Sealed Secret to be decrypted"
return health_status
