local hs = {}
if obj.status ~= nil then
    if obj.status.conditions ~= nil then
        for _, condition in pairs(obj.status.conditions) do
            if condition.type == "Degraded" and condition.status == "True" then 
                hs.status = "Degraded"
                hs.message = condition.message
                return hs
            elseif condition.type == "Progressing" and condition.status == "False" then
                hs.status = "Progressing"
                hs.message = condition.message
                return hs
            elseif condition.type == "Upgradeable" and condition.status == "True" then
                hs.status = "Healthy"
                hs.message = condition.message
                return hs
            elseif condition.type == "Available" and condition.status == "True" then
                hs.status = "Healthy"
                hs.message = condition.message
                return hs
            end
        end
    end
end

hs.status = "Progressing"
hs.message = "StorageCluster is still being initialized or is in an unknown state."
return hs