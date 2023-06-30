function getStatusBasedOnPhase(obj)
    hs = {}
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

if obj.spec.paused ~= nil and obj.spec.paused then
    hs = {}
    hs.status = "Suspended"
    hs.message = "Cluster is paused"
    return hs
end

hs = getStatusBasedOnPhase(obj)
if hs.status ~= "Healthy" then
    hs.message = getReadyContitionMessage(obj)
end

return hs