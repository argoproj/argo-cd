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
