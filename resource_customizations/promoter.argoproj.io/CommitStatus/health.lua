local hs = {}
hs.status = "Progressing"
hs.message = "Initializing commit status"

-- Check for deletion timestamp
if obj.metadata.deletionTimestamp then
    hs.status = "Progressing"
    hs.message = "Commit status is being deleted"
    return hs
end

-- Check if status exists
if not obj.status then
    return hs
end

-- Check for reconciliation conditions
local hasReadyCondition = false
if obj.status.conditions then
    for _, condition in ipairs(obj.status.conditions) do
        if condition.type == "Ready" then
            hasReadyCondition = true
            if condition.observedGeneration and obj.metadata.generation and condition.observedGeneration ~= obj.metadata.generation then
                hs.status = "Progressing"
                hs.message = "Waiting for commit status spec update to be observed"
                return hs
            end
            if condition.status == "False" and condition.reason == "ReconciliationError" then
                hs.status = "Degraded"
                hs.message = "Commit status reconciliation failed: " .. (condition.message or "Unknown error")
                return hs
            end
        end
    end
end
if not hasReadyCondition then
    hs.status = "Progressing"
    hs.message = "Commit status is not ready yet"
    return hs
end

if not obj.status.sha or not obj.status.phase then
    hs.status = "Healthy"
    hs.message = "Commit status is healthy"
    return hs
end

hs.status = "Healthy"
hs.message = "Commit status for commit " .. obj.status.sha .. " reports " .. obj.status.phase
return hs
