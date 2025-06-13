local hs_degraded = {}
local hs_progressing = {}
local hs_healthy = {}
local isDegraded = false
local isProgressing = false
local isHealthy = false
if obj.status ~= nil then
    if obj.status.conditions ~= nil then
        for _, condition in pairs(obj.status.conditions) do
            if condition.type == "Degraded" and condition.status == "True" then
                isDegraded = true
                hs.status = "Degraded"
                hs.message = condition.message
            elseif condition.type == "Progressing" and condition.status == "True" then
                isProgressing = true
                hs.status = "Progressing"
                hs.message = condition.message
            elseif condition.type == "Available" and condition.status == "True" then
                isHealthy = true
                hs.status = "Healthy"
                hs.message = condition.message
            end
        end
    end
end
if isDegraded then
    return hs_degraded
elseif isProgressing then
    return hs_progressing
elseif isHealthy then
    return hs_healthy
end
return { status = "Unknown", message = "StorageCluster is in an unknown state." }
