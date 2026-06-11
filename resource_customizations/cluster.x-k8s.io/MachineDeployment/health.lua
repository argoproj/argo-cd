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
hs.message = "Waiting for machines"

if obj.spec.paused ~= nil and obj.spec.paused then
    hs.status = "Suspended"
    hs.message = "MachineDeployment is paused"
    return hs
end

if obj.status ~= nil and obj.status.phase ~= nil then
    if obj.status.phase == "Running" then
        hs.status = "Healthy"
        hs.message = "Machines are running under this deployment"
    end
    if obj.status.phase == "ScalingUp" then
        hs.status = "Progressing"
        hs.message = "Cluster is spawning machines"
    end
    if obj.status.phase == "ScalingDown" then
        hs.status = "Progressing"
        hs.message = "Cluster is stopping machines"
    end
    if obj.status.phase == "Failed" then
        hs.status = "Degraded"
        hs.message = "MachineDeployment is failed"
    end
end

return hs