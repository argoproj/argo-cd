-- CRD spec: https://gitops-promoter.readthedocs.io/en/latest/crd-specs/#restoreactivecommit
-- https://gitops-promoter.readthedocs.io/en/latest/advanced-usage/rolling-back/

-- RestoreActiveCommit is healthy once the rollback has recorded status.activeSha.
-- spec.blockEnvironment defaults to true and is the steady state after a successful restore.

if obj.metadata and obj.metadata.deletionTimestamp then
    local hs = {}
    hs.status = "Progressing"
    hs.deletionMessage = "RestoreActiveCommit is being deleted"
    return hs
end
local hs = {}
hs.status = "Progressing"
hs.message = "Initializing active commit restore"


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
                hs.message = "Waiting for RestoreActiveCommit spec update to be observed"
                return hs
            end
            if condition.status == "False" then
                hs.status = "Degraded"
                local msg = condition.message or "Unknown error"
                local reason = condition.reason or "Unknown"
                if reason == "ReconciliationError" then
                    hs.message = "Active commit restore failed: " .. msg
                else
                    hs.message = "Active commit restore not ready (" .. reason .. "): " .. msg
                end
                return hs
            end
            if condition.status == "Unknown" then
                hs.status = "Progressing"
                local msg = condition.message or "Unknown"
                local reason = condition.reason or "Unknown"
                hs.message = "Active commit restore status unknown (" .. reason .. "): " .. msg
                return hs
            end
        end
    end
end

if not hasReadyCondition then
    hs.status = "Progressing"
    hs.message = "RestoreActiveCommit Ready condition is missing"
    return hs
end

local activeSha = obj.status.activeSha
if activeSha == nil or activeSha == "" then
    hs.status = "Progressing"
    hs.message = "Rollback has not completed"
    return hs
end

local branch = "environment"
if obj.spec and obj.spec.branch and obj.spec.branch ~= "" then
    branch = obj.spec.branch
end

local blocked = true
if obj.spec and obj.spec.blockEnvironment == false then
    blocked = false
end

hs.status = "Healthy"
if blocked then
    hs.message = "Environment " .. branch .. " is blocked at " .. activeSha
else
    hs.message = "Environment block released for " .. branch
end
return hs
