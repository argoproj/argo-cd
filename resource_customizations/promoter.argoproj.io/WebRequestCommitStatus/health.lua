local hs = {}
hs.status = "Progressing"
hs.message = "Initializing web request commit validation"

-- WebRequestCommitStatus (gitops-promoter v1alpha1): HTTP-based gating with per-environment phase
-- (pending / success / failure) in status.environments, plus aggregated Ready.

if obj.metadata.deletionTimestamp then
    hs.status = "Progressing"
    hs.message = "WebRequestCommitStatus is being deleted"
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
                hs.message = "Waiting for WebRequestCommitStatus spec update to be observed"
                return hs
            end
            if condition.status == "False" then
                hs.status = "Degraded"
                local msg = condition.message or "Unknown error"
                local reason = condition.reason or "Unknown"
                if reason == "ReconciliationError" then
                    hs.message = "Web request commit validation failed: " .. msg
                else
                    hs.message = "Web request commit validation not ready (" .. reason .. "): " .. msg
                end
                return hs
            end
            if condition.status == "Unknown" then
                hs.status = "Progressing"
                local msg = condition.message or "Unknown"
                local reason = condition.reason or "Unknown"
                hs.message = "Web request commit validation unknown (" .. reason .. "): " .. msg
                return hs
            end
        end
    end
end

if not hasReadyCondition then
    hs.status = "Progressing"
    hs.message = "WebRequestCommitStatus Ready condition is missing"
    return hs
end

local envs = obj.status.environments
if not envs or #envs == 0 then
    hs.status = "Healthy"
    hs.message = "Web request commit validation reconciled"
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
    elseif phase == "pending" then
        table.insert(pendingBranches, branch)
    elseif phase == "success" then
        successCount = successCount + 1
    else
        table.insert(pendingBranches, branch)
    end
end

if #failureBranches > 0 then
    hs.status = "Degraded"
    hs.message = "Web request validation failed for branch(es): " .. table.concat(failureBranches, ", ")
    return hs
end

if #pendingBranches > 0 then
    hs.status = "Progressing"
    hs.message = "Web request validation pending for branch(es): " .. table.concat(pendingBranches, ", ")
    return hs
end

hs.status = "Healthy"
hs.message = "Web request validation passed for " .. tostring(successCount) .. " environment(s)"
return hs
