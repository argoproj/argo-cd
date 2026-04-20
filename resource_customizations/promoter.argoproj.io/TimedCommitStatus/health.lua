local hs = {}
hs.status = "Progressing"
hs.message = "Initializing timed commit gate"

-- TimedCommitStatus (gitops-promoter v1alpha1): per-environment wait before reporting success.
-- status.environments[].phase is pending or success (see API).

if obj.metadata.deletionTimestamp then
    hs.status = "Progressing"
    hs.message = "TimedCommitStatus is being deleted"
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
                hs.message = "Waiting for TimedCommitStatus spec update to be observed"
                return hs
            end
            if condition.status == "False" then
                hs.status = "Degraded"
                local msg = condition.message or "Unknown error"
                local reason = condition.reason or "Unknown"
                if reason == "ReconciliationError" then
                    hs.message = "Timed commit gate failed: " .. msg
                else
                    hs.message = "Timed commit gate not ready (" .. reason .. "): " .. msg
                end
                return hs
            end
            if condition.status == "Unknown" then
                hs.status = "Progressing"
                local msg = condition.message or "Unknown"
                local reason = condition.reason or "Unknown"
                hs.message = "Timed commit gate status unknown (" .. reason .. "): " .. msg
                return hs
            end
        end
    end
end

if not hasReadyCondition then
    hs.status = "Progressing"
    hs.message = "TimedCommitStatus Ready condition is missing"
    return hs
end

local envs = obj.status.environments
if not envs or #envs == 0 then
    hs.status = "Healthy"
    hs.message = "Timed commit gate reconciled"
    return hs
end

local pendingBranches = {}
local failureBranches = {}
local successCount = 0

for _, env in ipairs(envs) do
    local branch = env.branch or "?"
    local phase = env.phase or "pending"
    if phase == "failure" then
        table.insert(failureBranches, branch)
    elseif phase == "success" then
        successCount = successCount + 1
    else
        table.insert(pendingBranches, branch)
    end
end

if #failureBranches > 0 then
    hs.status = "Degraded"
    hs.message = "Timed commit gate failed for branch(es): " .. table.concat(failureBranches, ", ")
    return hs
end

if #pendingBranches > 0 then
    hs.status = "Progressing"
    hs.message = "Timed commit gate pending for branch(es): " .. table.concat(pendingBranches, ", ")
    return hs
end

hs.status = "Healthy"
hs.message = "Timed commit gate satisfied for " .. tostring(successCount) .. " environment(s)"
return hs
