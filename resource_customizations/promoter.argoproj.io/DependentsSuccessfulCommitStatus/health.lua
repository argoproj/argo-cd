-- CRD spec: https://gitops-promoter.readthedocs.io/en/latest/crd-specs/#dependentssuccessfulcommitstatus

-- DependentsSuccessfulCommitStatus reports each environment in status.environments[].
-- phase is pending, success, or failure when a child CommitStatus exists.
-- upstreams[].satisfied is false while a dependency has not been promoted.

if obj.metadata and obj.metadata.deletionTimestamp then
    local hs = {}
    hs.status = "Progressing"
    hs.deletionMessage = "DependentsSuccessfulCommitStatus is being deleted"
    return hs
end
local hs = {}
hs.status = "Progressing"
hs.message = "Initializing dependents gate"


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
                hs.message = "Waiting for DependentsSuccessfulCommitStatus spec update to be observed"
                return hs
            end
            if condition.status == "False" then
                hs.status = "Degraded"
                local msg = condition.message or "Unknown error"
                local reason = condition.reason or "Unknown"
                if reason == "ReconciliationError" then
                    hs.message = "Dependents gate failed: " .. msg
                else
                    hs.message = "Dependents gate not ready (" .. reason .. "): " .. msg
                end
                return hs
            end
            if condition.status == "Unknown" then
                hs.status = "Progressing"
                local msg = condition.message or "Unknown"
                local reason = condition.reason or "Unknown"
                hs.message = "Dependents gate status unknown (" .. reason .. "): " .. msg
                return hs
            end
        end
    end
end

if not hasReadyCondition then
    hs.status = "Progressing"
    hs.message = "DependentsSuccessfulCommitStatus Ready condition is missing"
    return hs
end

local envs = obj.status.environments
if not envs or #envs == 0 then
    hs.status = "Healthy"
    hs.message = "Dependents gate reconciled"
    return hs
end

local pendingBranches = {}
local failureBranches = {}
local waitingBranches = {}
local successCount = 0

for _, env in ipairs(envs) do
    local branch = env.branch or "?"
    local phase = env.phase
    if phase == "failure" then
        table.insert(failureBranches, branch)
    else
        local waiting = false
        if env.upstreams then
            for _, upstream in ipairs(env.upstreams) do
                if upstream.satisfied == false then
                    waiting = true
                    break
                end
            end
        end
        if phase == "pending" then
            table.insert(pendingBranches, branch)
        elseif waiting then
            table.insert(waitingBranches, branch)
        elseif phase == nil or phase == "" or phase == "success" then
            successCount = successCount + 1
        else
            table.insert(pendingBranches, branch)
        end
    end
end

if #failureBranches > 0 then
    hs.status = "Degraded"
    hs.message = "Dependents gate failed for branch(es): " .. table.concat(failureBranches, ", ")
    return hs
end

if #pendingBranches > 0 then
    hs.status = "Progressing"
    hs.message = "Dependents gate pending for branch(es): " .. table.concat(pendingBranches, ", ")
    return hs
end

if #waitingBranches > 0 then
    hs.status = "Progressing"
    hs.message = "Dependents gate waiting on upstreams for branch(es): " .. table.concat(waitingBranches, ", ")
    return hs
end

hs.status = "Healthy"
hs.message = "Dependents gate satisfied for " .. tostring(successCount) .. " environment(s)"
return hs
