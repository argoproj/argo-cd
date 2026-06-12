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
  if obj.status.readyReplicas ~= 0 and obj.status.readyReplicas == obj.status.replicas then
    health_status.status = "Healthy"
    health_status.message = "All ZK Nodes have joined the ensemble"
    return health_status
  end
end
health_status.status = "Progressing"
health_status.message = "Waiting for ZK Nodes to join the ensemble"
return health_status