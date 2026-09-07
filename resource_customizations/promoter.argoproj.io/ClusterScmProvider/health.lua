-- CRD spec: https://gitops-promoter.readthedocs.io/en/latest/crd-specs/#clusterscmprovider
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
hs.message = "Initializing cluster SCM provider"

-- ClusterScmProvider (gitops-promoter v1alpha1): cluster-scoped SCM provider; same Ready semantics as ScmProvider.

if obj.metadata.deletionTimestamp then
    hs.status = "Progressing"
    hs.message = formatDeletingWithFinalizers(
        "ClusterScmProvider is being deleted.",
        obj.metadata.finalizers,
        {
            ["clusterscmprovider.promoter.argoproj.io/finalizer"] = {
                wait = "Waiting until no GitRepository still references this cluster-wide SCM provider.",
                risk = "GitRepository objects can reference a provider that no longer exists.",
            },
        }
    )
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
                hs.message = "Waiting for ClusterScmProvider spec update to be observed"
                return hs
            end
            if condition.status == "False" then
                hs.status = "Degraded"
                local msg = condition.message or "Unknown error"
                local reason = condition.reason or "Unknown"
                if reason == "ReconciliationError" then
                    hs.message = "Cluster SCM provider validation failed: " .. msg
                else
                    hs.message = "Cluster SCM provider not ready (" .. reason .. "): " .. msg
                end
                return hs
            end
            if condition.status == "Unknown" then
                hs.status = "Progressing"
                local msg = condition.message or "Unknown"
                local reason = condition.reason or "Unknown"
                hs.message = "Cluster SCM provider readiness unknown (" .. reason .. "): " .. msg
                return hs
            end
        end
    end
end

if not hasReadyCondition then
    hs.status = "Progressing"
    hs.message = "ClusterScmProvider Ready condition is missing"
    return hs
end

hs.status = "Healthy"
hs.message = "Cluster SCM provider is ready"
return hs
