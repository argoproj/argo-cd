local hs = {}

hs.status = "Progressing"
hs.message = ""

if obj.status ~= nil and obj.status.currentHealthy ~= nil then
    if obj.status.expectedMachines == obj.status.currentHealthy then
        hs.status = "Healthy"
    else
        hs.status = "Degraded"
    end
end

return hs
