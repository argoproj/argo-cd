hs = {}
if obj.status ~= nil then
    if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
        if condition.type == "Ready" and condition.status == "False" then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
        end
        if condition.type == "Stalled" and condition.status == "True" then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
        end
        if condition.type == "Ready" and condition.status == "True" then
        if obj.status.replicas ~= nil and obj.status.replicas > 0 then
            hs.status = "Healthy"
            hs.message = condition.message
        else
            hs.status = "Suspended"
            hs.message = "No replicas available"
        end
        return hs
        end
    end
    end
end

hs.status = "Progressing"
hs.message = "Waiting for Function"
return hs
