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

function getStatusBasedOnPhase(obj)
    local hs = {}
    hs.status = "Progressing"
    hs.message = "Waiting for machines"
    if obj.status ~= nil and obj.status.phase ~= nil then
        if obj.status.phase == "Running" then
            hs.status = "Healthy"
            hs.message = "Machine is running"
        end
        if obj.status.phase == "Failed" then
            hs.status = "Degraded"
            hs.message = ""
        end
    end
    return hs
end

function getReadyContitionMessage(obj)
    if obj.status ~= nil and obj.status.conditions ~= nil then
        for i, condition in ipairs(obj.status.conditions) do
        if condition.type == "Ready" and condition.status == "False" then
            return condition.message
        end
        end
    end
    return "Condition is unknown"
end

local hs = getStatusBasedOnPhase(obj)
if hs.status ~= "Healthy" then
    hs.message = getReadyContitionMessage(obj)
end

return hs