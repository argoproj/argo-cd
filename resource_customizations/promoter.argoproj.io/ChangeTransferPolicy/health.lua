local hs = {}
hs.status = "Progressing"
hs.message = "Initializing change transfer policy"

-- Check for deletion timestamp
if obj.metadata.deletionTimestamp then
    hs.status = "Progressing"
    hs.message = "Change transfer policy is being deleted"
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
                hs.message = "Waiting for change transfer policy spec update to be observed"
                return hs
            end
            if condition.status == "False" and condition.reason == "ReconciliationError" then
                hs.status = "Degraded"
                hs.message = "Change transfer policy reconciliation failed: " .. (condition.message or "Unknown error")
                return hs
            end
        end
    end
end
if not hasReadyCondition then
    hs.status = "Progressing"
    hs.message = "Change transfer policy is not ready yet"
    return hs
end

if not obj.status.active or not obj.status.active.dry or not obj.status.active.dry.sha or obj.status.active.dry.sha == "" then
    hs.status = "Progressing"
    hs.message = "The active commit DRY SHA is missing or empty."
    return hs
end
if not obj.status.proposed or not obj.status.proposed.dry or not obj.status.proposed.dry.sha or obj.status.proposed.dry.sha == "" then
    hs.status = "Progressing"
    hs.message = "The proposed commit DRY SHA is missing or empty."
    return hs
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

if obj.status.proposed.dry.sha ~= obj.status.active.dry.sha then
    local pendingCount = 0
    local successCount = 0
    local failureCount = 0
    local pendingKeys = {}
    local failedKeys = {}

    for _, status in ipairs(obj.status.proposed.commitStatuses or {}) do
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
        "Promotion in progress from '" .. getShortSha(obj.status.active.dry.sha) ..
        "' to '" .. getShortSha(obj.status.proposed.dry.sha) .. "': " ..
        pendingCount .. " pending, " .. successCount .. " successful, " .. failureCount .. " failed. "

    if pendingCount > 0 then
        hs.message = hs.message .. "Pending commit statuses: " .. table.concat(pendingKeys, ", ") .. ". "
    end
    if failureCount > 0 then
        hs.message = hs.message .. "Failed commit statuses: " .. table.concat(failedKeys, ", ") .. ". "
    end
    return hs
end

local pendingCount = 0
local failureCount = 0
local successCount = 0
local pendingKeys = {}
local failedKeys = {}
for _, status in ipairs(obj.status.active.commitStatuses or {}) do
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
if pendingCount > 0 or failureCount > 0 then
    hs.status = "Healthy"
    hs.message = "Environment is up-to-date, but there are non-successful active commit statuses: " .. pendingCount .. " pending, " .. successCount .. " successful, " .. failureCount .. " failed. "
    if pendingCount > 0 then
        hs.message = hs.message .. "Pending commit statuses: " .. table.concat(pendingKeys, ", ") .. ". "
    end
    if failureCount > 0 then
        hs.message = hs.message .. "Failed commit statuses: " .. table.concat(failedKeys, ", ") .. ". "
    end
    return hs
end

hs.status = "Healthy"
hs.message = "Environment is up-to-date on commit " .. getShortSha(obj.status.active.dry.sha) .. "."
return hs