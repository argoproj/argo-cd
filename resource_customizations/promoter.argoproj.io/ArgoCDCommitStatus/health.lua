local hs = {}
hs.status = "Progressing"
hs.message = "Initializing Argo CD commit status"

-- Check for deletion timestamp
if obj.metadata.deletionTimestamp then
    hs.status = "Progressing"
    hs.message = "Argo CD commit status is being deleted"
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
                hs.message = "Waiting for Argo CD commit status spec update to be observed"
                return hs
            end
            -- Check for any False condition status
            if condition.status == "False" then
                hs.status = "Degraded"
                local msg = condition.message or "Unknown error"
                local reason = condition.reason or "Unknown"
                -- Don't include ReconciliationError in the message since it's redundant
                if reason == "ReconciliationError" then
                    hs.message = "Argo CD commit status reconciliation failed: " .. msg
                else
                    hs.message = "Argo CD commit status reconciliation failed (" .. reason .. "): " .. msg
                end
                return hs
            end
        end
    end
end
if not hasReadyCondition then
    hs.status = "Progressing"
    hs.message = "Argo CD commit status is not ready yet"
    return hs
end

if not obj.status.applicationsSelected or #obj.status.applicationsSelected == 0 then
    hs.status = "Degraded"
    hs.message = "Argo CD commit status has no applications configured"
    return hs
end

hs.status = "Healthy"
hs.message = "Argo CD commit status is healthy and is tracking " .. #obj.status.applicationsSelected .. " applications"
return hs
