local hs = {}

-- Check if CRD is terminating
if obj.metadata.deletionTimestamp ~= nil then
    hs.status = "Progressing"
    hs.message = "CRD is terminating"
    return hs
end

if obj.status.conditions == nil then
    hs.status = "Progressing"
    hs.message = "Status conditions not found"
    return hs
end

if #obj.status.conditions == 0 then
    hs.status = "Progressing"
    hs.message = "Status conditions not found"
    return hs
end

local isEstablished
local conditionMsg = ""

for _, condition in pairs(obj.status.conditions) do

    -- Check if CRD is terminating
    if condition.type == "Terminating" and condition.status == "True" then
        hs.status = "Progressing"
        hs.message = "CRD is terminating: " .. condition.message
        return hs
    end

    -- Check if K8s has accepted names for this CRD
    if condition.type == "NamesAccepted" and condition.status == "False" then
        hs.status = "Degraded"
        hs.message = "CRD names have not been accepted: " .. condition.message
        return hs
    end

    -- Checking if CRD has violations
    if condition.type == "NonStructuralSchema" and condition.status == "True" then
        hs.status = "Degraded"
        hs.message = "Schema violations found: " .. condition.message
        return hs
    end

    -- Checking if CRD is established
    if condition.type == "Established" then
        if condition.status == "True" then
            isEstablished = true
            conditionMsg = condition.message
        elseif condition.reason == "Installing" then
            hs.status = "Progressing"
            hs.message = "CRD is being installed"
            return hs
        end
    end
end

if not isEstablished then
    hs.status = "Degraded"
    hs.message = "CRD is not established"
    return hs
end

hs.status = "Healthy"
hs.message = "CRD is healthy"
return hs