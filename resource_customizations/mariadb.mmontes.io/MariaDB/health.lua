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

if obj.status ~= nil and obj.status.conditions ~= nil then

    for i, condition in ipairs(obj.status.conditions) do

        health_status.message = condition.message

        if condition.status == "False" then
            if condition.reason == "Failed" then
                health_status.status = "Degraded"
                return health_status
            end
            health_status.status = "Progressing"
            return health_status
        end
    end

    health_status.status = "Healthy"
    return health_status
end

health_status.status = "Progressing"
health_status.message = "No status info available"
return health_status
