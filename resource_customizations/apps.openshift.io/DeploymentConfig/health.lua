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

local health_check = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil and obj.status.replicas ~= nil then
    local numTrue = 0
    for i, condition in pairs(obj.status.conditions) do
      if (condition.type == "Available" or (condition.type == "Progressing" and condition.reason == "NewReplicationControllerAvailable")) and condition.status == "True" then
        numTrue = numTrue + 1
      end
    end
    if numTrue == 2 or obj.status.replicas == 0 then
      health_check.status = "Healthy"
      health_check.message = "replication controller successfully rolled out"
      return health_check
    elseif numTrue == 1 then
      health_check.status = "Progressing"
      health_check.message = "replication controller is waiting for pods to run"
      return health_check
    else
      health_check.status = "Degraded"
      health_check.message = "Deployment config is degraded"
      return health_check
    end
  end
end
health_check.status = "Progressing"
health_check.message = "replication controller is waiting for pods to run"
return health_check