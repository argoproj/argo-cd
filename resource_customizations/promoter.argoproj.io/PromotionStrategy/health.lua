local hs = {}
hs.status = "Progressing"
hs.message = "Initializing promotion strategy"

-- Check for deletion timestamp
if obj.metadata.deletionTimestamp then
    hs.status = "Progressing"
    hs.message = "Promotion strategy is being deleted"
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
            -- Check observedGeneration vs metadata.generation within the reconciliation condition
            if condition.observedGeneration and obj.metadata.generation and condition.observedGeneration ~= obj.metadata.generation then
                hs.status = "Progressing"
                hs.message = "Waiting for promotion strategy spec update to be observed"
                return hs
            end
            if condition.status == "False" and condition.reason == "ReconciliationError" then
                hs.status = "Degraded"
                hs.message = "Promotion strategy reconciliation failed: " .. (condition.message or "Unknown error")
                return hs
            end
        end
    end
end

-- If no Ready condition is found, return Progressing status
if not hasReadyCondition then
    hs.status = "Progressing"
    hs.message = "Promotion strategy is not ready yet"
    return hs
end

if not obj.status.environments or #obj.status.environments == 0 then
    hs.status = "Degraded"
    hs.message = "Promotion strategy has no environments configured"
    return hs
end

-- Make sure there's a fully-populated status for both active and proposed commits in all environments. If anything is
-- missing or empty, return a Progressing status.
for _, env in ipairs(obj.status.environments) do
    if not env.active or not env.active.dry or not env.active.dry.sha or env.active.dry.sha == "" then
        hs.status = "Progressing"
        hs.message = "The active commit DRY SHA is missing or empty in environment '" .. env.branch .. "'."
        return hs
    end
    if not env.proposed or not env.proposed.dry or not env.proposed.dry.sha or env.proposed.dry.sha == "" then
        hs.status = "Progressing"
        hs.message = "The proposed commit DRY SHA is missing or empty in environment '" .. env.branch .. "'."
        return hs
    end
end

-- Check if all the proposed environments have the same proposed commit dry sha. If not, return a Progressing status.
local proposedSha = obj.status.environments[1].proposed.dry.sha  -- Don't panic, Lua is 1-indexed.
for _, env in ipairs(obj.status.environments) do
    if env.proposed.dry.sha ~= proposedSha then
        hs.status = "Progressing"
        hs.status = "Not all environments have the same proposed commit SHA. This likely means the hydrator has not run for all environments yet."
        return hs
    end
end

-- Helper function to get short SHA
local function getShortSha(sha)
    if not sha or sha == "" then
        return ""
    end
    if string.len(sha) > 7 then
        return string.sub(sha, 1, 7)
    end
    return sha
end

-- Find the first environment with a proposed commit dry sha that doesn't match the active one. Loop over its commit
-- statuses and build a summary about how many are pending, successful, or failed. Return a Progressing status for this
-- in-progress environment.
for _, env in ipairs(obj.status.environments) do
    if env.proposed.dry.sha ~= env.active.dry.sha then
        local pendingCount = 0
        local successCount = 0
        local failureCount = 0
        local pendingKeys = {}
        local failedKeys = {}

        -- pending, success, and failure are the only possible values
        -- https://github.com/argoproj-labs/gitops-promoter/blob/c58d55ef52f86ff84e4f8fa35d2edba520886e3b/api/v1alpha1/commitstatus_types.go#L44
        for _, status in ipairs(env.proposed.commitStatuses or {}) do
            if status.phase == "pending" then
                pendingCount = pendingCount + 1
                table.insert(pendingKeys, status.key)
            elseif status.phase == "success" then
                successCount = successCount + 1
            elseif status.phase == "failure" then
                failureCount = failureCount + 1
                table.insert(failedKeys, status.key)
            end
        end

        hs.status = "Progressing"
        hs.message =
            "Promotion in progress for environment '" .. env.branch ..
            "' from '" .. getShortSha(env.active.dry.sha) ..
            "' to '" .. getShortSha(env.proposed.dry.sha) .. "': " ..
            pendingCount .. " pending, " .. successCount .. " successful, " .. failureCount .. " failed. "

        if pendingCount > 0 then
            hs.message = hs.message .. "Pending commit statuses: " .. table.concat(pendingKeys, ", ") .. ". "
        end
        if failureCount > 0 then
            hs.message = hs.message .. "Failed commit statuses: " .. table.concat(failedKeys, ", ") .. ". "
        end
        return hs
    end
end

-- Check all environments for active commit statuses that aren't successful. For each environment with a non-successful
-- commit status, get a count of how many aren't successful. Write a summary of non-successful environments.
local nonSuccessfulEnvironments = {}
for _, env in ipairs(obj.status.environments) do
    local pendingCount = 0
    local failureCount = 0

    for _, status in ipairs(env.active.commitStatuses or {}) do
        if status.phase == "pending" then
            pendingCount = pendingCount + 1
        elseif status.phase == "failure" then
            failureCount = failureCount + 1
        end
    end

    if pendingCount > 0 or failureCount > 0 then
        nonSuccessfulEnvironments[env.branch] = {
            pending = pendingCount,
            failure = failureCount,
        }
    end
end

if next(nonSuccessfulEnvironments) then
    local envMessages = {}
    for branch, counts in pairs(nonSuccessfulEnvironments) do
        local msg = branch .. " (" .. counts.failure .. " failed, " .. counts.pending .. " pending)"
        table.insert(envMessages, msg)
    end
    hs.status = "Healthy"
    hs.message = "Environments are up-to-date, but some environments have non-successful commit statuses: " .. table.concat(envMessages, ", ") .. "."
    return hs
end


-- If all environments have the same proposed commit dry sha as the active one, we can consider the promotion strategy
-- healthy. This means all environments are in sync and no further action is needed.
hs.status = "Healthy"
hs.message = "All environments are up-to-date on commit '" .. getShortSha(obj.status.environments[1].proposed.dry.sha) .. "'."
return hs
