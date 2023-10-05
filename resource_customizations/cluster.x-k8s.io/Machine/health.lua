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