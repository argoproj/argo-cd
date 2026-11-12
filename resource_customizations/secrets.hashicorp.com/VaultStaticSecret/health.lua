local hs = {
    status = "Progressing",
    message = "Waiting for VaultStaticSecret to be synced"
}

if obj.metadata == nil or obj.metadata.generation == nil or obj.status == nil then
    return hs
end

-- Vault Secrets Operator 1.0.0 and later set the Healthy condition on every status update. Ready was added in 1.3.0
-- and always has the same status.
-- https://github.com/hashicorp/vault-secrets-operator/blob/v1.6.2/controllers/vaultstaticsecret_controller.go
if obj.status.conditions ~= nil then
    local healthy
    local synced
    for _, condition in ipairs(obj.status.conditions) do
        if condition.type == "Healthy" then
            healthy = condition
        elseif condition.type == "SecretSynced" then
            synced = condition
        end
    end
    -- The conditions can still describe an earlier generation.
    if healthy == nil or healthy.observedGeneration ~= obj.metadata.generation then
        return hs
    end
    if healthy.status == "True" then
        hs.status = "Healthy"
        hs.message = "Secret synced"
    elseif healthy.status == "False" then
        hs.status = "Degraded"
        hs.message = "Failed to sync the secret"
    end
    -- SecretSynced carries the sync error. Before 1.6.0 it isn't updated when the secret data hasn't changed, so it
    -- can still describe an earlier failure.
    if synced ~= nil and synced.status == healthy.status then
        hs.message = synced.message
    end
    return hs
end

-- Earlier versions set no conditions and only update the status after a successful sync.
if obj.status.lastGeneration == obj.metadata.generation then
    hs.status = "Healthy"
    hs.message = "Secret synced"
end
return hs
