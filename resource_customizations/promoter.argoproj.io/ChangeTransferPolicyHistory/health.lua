-- CRD spec: https://gitops-promoter.readthedocs.io/en/latest/crd-specs/#changetransferpolicyhistory

-- ChangeTransferPolicyHistory records recent promotions. Reconciliation health is the Ready
-- condition. A history entry with mergeCommitSnapshotMismatch is still a successful reconcile.

if obj.metadata and obj.metadata.deletionTimestamp then
    local hs = {}
    hs.status = "Progressing"
    hs.deletionMessage = "Promotion history is being deleted"
    return hs
end
local hs = {}
hs.status = "Progressing"
hs.message = "Initializing promotion history"


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
                hs.message = "Waiting for ChangeTransferPolicyHistory spec update to be observed"
                return hs
            end
            if condition.status == "False" then
                hs.status = "Degraded"
                local msg = condition.message or "Unknown error"
                local reason = condition.reason or "Unknown"
                if reason == "ReconciliationError" then
                    hs.message = "Promotion history failed: " .. msg
                else
                    hs.message = "Promotion history not ready (" .. reason .. "): " .. msg
                end
                return hs
            end
            if condition.status == "Unknown" then
                hs.status = "Progressing"
                local msg = condition.message or "Unknown"
                local reason = condition.reason or "Unknown"
                hs.message = "Promotion history status unknown (" .. reason .. "): " .. msg
                return hs
            end
        end
    end
end

if not hasReadyCondition then
    hs.status = "Progressing"
    hs.message = "ChangeTransferPolicyHistory Ready condition is missing"
    return hs
end

hs.status = "Healthy"
hs.message = "Promotion history is ready"
return hs
