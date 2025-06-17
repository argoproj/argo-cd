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
local isTerminating
local namesNotAccepted
local hasViolations
local conditionMsg = ""

for _, condition in pairs(obj.status.conditions) do

    -- Check if CRD is terminating
    if condition.type == "Terminating" and condition.status == "True" then
        isTerminating = true
        conditionMsg = condition.message
    end

    -- Check if K8s has accepted names for this CRD
    if condition.type == "NamesAccepted" and condition.status == "False" then
        namesNotAccepted = true
        conditionMsg = condition.message
    end

    -- Checking if CRD is established
    if condition.type == "Established" and condition.status == "True" then
        isEstablished = true
        conditionMsg = condition.message
    end

    -- Checking if CRD has violations
    if condition.type == "NonStructuralSchema" and condition.status == "True" then
        hasViolations = true
        conditionMsg = condition.message
    end

end

if isTerminating then
    hs.status = "Progressing"
    hs.message = "CRD is terminating: " .. conditionMsg
    return hs
end

if namesNotAccepted then
    hs.status = "Degraded"
    hs.message = "CRD names have not been accepted: " .. conditionMsg
    return hs
end

if not isEstablished then
    hs.status = "Degraded"
    hs.message = "CRD is not established"
    return hs
end

if hasViolations then
    hs.status = "Degraded"
    hs.message = "Schema violations found: " .. conditionMsg
    return hs
end

hs.status = "Healthy"
hs.message = "CRD is healthy"
return hs