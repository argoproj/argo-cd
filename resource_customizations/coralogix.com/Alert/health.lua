hs = {}

hs.status = "Progressing"
hs.message = "Waiting for status to be updated"

if obj.status ~= nil and obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
    if condition.type == "RemoteSynced" then
        if condition.status == "True" then
        hs.status = "Healthy"
        hs.message = "Resource is ready"
        return hs
        elseif condition.status == "False" then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
        end
    end
    end
end
return hs
