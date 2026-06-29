-- CRD spec: https://gitops-promoter.readthedocs.io/en/latest/crd-specs/#pullrequest
-- Promoter finalizers: https://gitops-promoter.readthedocs.io/en/latest/debugging/finalizers/

local function formatDeletingWithFinalizers(base, finalizers, catalog)
    if not finalizers then
        return base
    end
    local parts = { base }
    for _, f in ipairs(finalizers) do
        local e = catalog[f]
        if e then
            table.insert(parts, f .. ": " .. e.wait .. " Risk if removed manually: " .. e.risk)
        end
    end
    return table.concat(parts, " ")
end

local hs = {}
hs.status = "Progressing"
hs.message = "Initializing pull request"

-- Check for deletion timestamp
if obj.metadata.deletionTimestamp then
    hs.status = "Progressing"
    hs.message = formatDeletingWithFinalizers(
        "Pull request is being deleted.",
        obj.metadata.finalizers,
        {
            ["pullrequest.promoter.argoproj.io/finalizer"] = {
                wait = "Waiting for SCM closure or reconciliation of the pull request.",
                risk = "the real PR can stay open after the CR is gone.",
            },
            ["changetransferpolicy.promoter.argoproj.io/pullrequest-finalizer"] = {
                wait = "Waiting for the ChangeTransferPolicy to observe PR identity and state.",
                risk = "promotion status or history can be wrong or racy.",
            },
        }
    )
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
        if condition.type == "Reconciled" or condition.type == "Ready" then
            hasReadyCondition = true
            -- Check observedGeneration vs metadata.generation within the reconciliation condition
            if condition.observedGeneration and obj.metadata.generation and condition.observedGeneration ~= obj.metadata.generation then
                hs.status = "Progressing"
                hs.message = "Waiting for pull request spec update to be observed"
                return hs
            end
            if condition.status == "False" then
                hs.status = "Degraded"
                local msg = condition.message or "Unknown error"
                local reason = condition.reason or "Unknown"
                -- Don't include ReconciliationError in the message since it's redundant
                if reason == "ReconciliationError" then
                    hs.message = "Pull request reconciliation failed: " .. msg
                else
                    hs.message = "Pull request reconciliation failed (" .. reason .. "): " .. msg
                end
                return hs
            end
        end
    end
end
if not hasReadyCondition then
    hs.status = "Progressing"
    hs.message = "Pull request is not ready yet"
    return hs
end

-- This shouldn't happen, but if the condition says reconciliation succeeded, just trust it.
if not obj.status.id or not obj.status.state then
    hs.status = "Healthy"
    hs.message = "Pull request is healthy"
    return hs
end

hs.status = "Healthy"
hs.message = "Pull request is " .. obj.status.state .. " as PR " .. obj.status.id
return hs
