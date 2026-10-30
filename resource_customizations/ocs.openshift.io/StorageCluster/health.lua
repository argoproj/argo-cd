local hs_degraded = {}
local hs_progressing = {}
local hs_healthy = {}
local is_degraded = false
local is_progressing = false
local is_healthy = false
if obj ~= nil and obj.status ~= nil and obj.status.conditions ~= nil then
    for _, condition in pairs(obj.status.conditions) do
        if condition.type == "Degraded" and condition.status == "True" then
            is_degraded = true
            hs_degraded.status = "Degraded"
            hs_degraded.message = condition.message
        elseif condition.type == "Progressing" and condition.status == "True" then
            is_progressing = true
            hs_progressing.status = "Progressing"
            hs_progressing.message = condition.message
        elseif condition.type == "Available" and condition.status == "True" then
            is_healthy = true
            hs_healthy.status = "Healthy"
            hs_healthy.message = condition.message
        end
    end
end
if is_degraded then
    return hs_degraded
elseif is_progressing then
    return hs_progressing
elseif is_healthy then
    return hs_healthy
end
return { status = "Unknown", message = "StorageCluster is in an unknown state." }
