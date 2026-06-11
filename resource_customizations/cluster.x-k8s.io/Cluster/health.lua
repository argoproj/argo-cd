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

function getStatusBasedOnPhase(obj, hs)
    hs.status = "Progressing"
    hs.message = "Waiting for clusters"
    if obj.status ~= nil and obj.status.phase ~= nil then
        if obj.status.phase == "Provisioned" then
            hs.status = "Healthy"
            hs.message = "Cluster is running"
        end
        if obj.status.phase == "Failed" then
            hs.status = "Degraded"
            hs.message = ""
        end
    end
    return hs
end

function getReadyContitionStatus(obj, hs)
    if obj.status ~= nil and obj.status.conditions ~= nil then
        for i, condition in ipairs(obj.status.conditions) do
        if condition.type == "Ready" and condition.status == "False" then
            hs.status = "Degraded"
            hs.message = condition.message
            return hs
        end
        end
    end
    return hs
end

local hs = {}
if obj.spec.paused ~= nil and obj.spec.paused then
    hs.status = "Suspended"
    hs.message = "Cluster is paused"
    return hs
end

getStatusBasedOnPhase(obj, hs)
getReadyContitionStatus(obj, hs)

return hs
