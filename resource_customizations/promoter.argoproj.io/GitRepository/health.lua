local hs = {}
hs.status = "Progressing"
hs.message = "Initializing Git repository"

-- GitRepository (gitops-promoter v1alpha1): repo reference validated via standard Ready condition.

if obj.metadata.deletionTimestamp then
    hs.status = "Progressing"
    hs.message = "GitRepository is being deleted"
    return hs
end

if not obj.status then
    return hs
end

local hasReadyCondition = false
if obj.status.conditions then
    for _, condition in ipairs(obj.status.conditions) do
        if condition.type == "Ready" then
            hasReadyCondition = true
            if condition.observedGeneration and obj.metadata.generation and condition.observedGeneration ~= obj.metadata.generation then
                hs.status = "Progressing"
                hs.message = "Waiting for GitRepository spec update to be observed"
                return hs
            end
            if condition.status == "False" then
                hs.status = "Degraded"
                local msg = condition.message or "Unknown error"
                local reason = condition.reason or "Unknown"
                if reason == "ReconciliationError" then
                    hs.message = "Git repository validation failed: " .. msg
                else
                    hs.message = "Git repository not ready (" .. reason .. "): " .. msg
                end
                return hs
            end
            if condition.status == "Unknown" then
                hs.status = "Progressing"
                local msg = condition.message or "Unknown"
                local reason = condition.reason or "Unknown"
                hs.message = "Git repository readiness unknown (" .. reason .. "): " .. msg
                return hs
            end
        end
    end
end

if not hasReadyCondition then
    hs.status = "Progressing"
    hs.message = "GitRepository Ready condition is missing"
    return hs
end

hs.status = "Healthy"
hs.message = "Git repository is ready"
return hs
