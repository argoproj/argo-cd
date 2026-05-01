-- CRD spec: https://gitops-promoter.readthedocs.io/en/latest/crd-specs/

local hs = {}
hs.status = "Progressing"
hs.message = "Initializing git commit validation"

-- GitCommitStatus (gitops-promoter) reports per-environment validation in status.environments
-- (branch, phase, proposedHydratedSha, targetedSha, expressionResult). It is not CommitStatus:
-- there is no top-level status.sha / status.phase.

if obj.metadata.deletionTimestamp then
    hs.status = "Progressing"
    hs.message = "GitCommitStatus is being deleted"
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
                hs.message = "Waiting for GitCommitStatus spec update to be observed"
                return hs
            end
            if condition.status == "False" then
                hs.status = "Degraded"
                local msg = condition.message or "Unknown error"
                local reason = condition.reason or "Unknown"
                if reason == "ReconciliationError" then
                    hs.message = "Git commit validation failed: " .. msg
                else
                    hs.message = "Git commit validation not ready (" .. reason .. "): " .. msg
                end
                return hs
            end
            if condition.status == "Unknown" then
                hs.status = "Progressing"
                local msg = condition.message or "Unknown"
                local reason = condition.reason or "Unknown"
                hs.message = "Git commit validation status unknown (" .. reason .. "): " .. msg
                return hs
            end
        end
    end
end

if not hasReadyCondition then
    hs.status = "Progressing"
    hs.message = "GitCommitStatus Ready condition is missing"
    return hs
end

local envs = obj.status.environments
if not envs or #envs == 0 then
    hs.status = "Healthy"
    hs.message = "Git commit validation reconciled"
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
    hs.message = "Git commit validation failed for branch(es): " .. table.concat(failureBranches, ", ")
    return hs
end

if #pendingBranches > 0 then
    hs.status = "Progressing"
    hs.message = "Git commit validation pending for branch(es): " .. table.concat(pendingBranches, ", ")
    return hs
end

hs.status = "Healthy"
hs.message = "Git commit validation passed for " .. tostring(successCount) .. " environment(s)"
return hs
