local hs = {}

local cnpgStatus = {
    ["Cluster in healthy state"] = "Healthy",
    ["Setting up primary"] = "Progressing",
    ["Setting up primary"] = "Progressing",
    ["Creating a new replica"] = "Progressing",
    ["Upgrading cluster"] = "Progressing",
    ["Waiting for the instances to become active"] = "Progressing",
    ["Promoting to primary cluster"] = "Progressing",
    ["Switchover in progress"] = "Degraded",
    ["Failing over"] = "Degraded",
    ["Upgrading Postgres major version"] = "Degraded",
    ["Cluster upgrade delayed"] = "Degraded",
    ["Waiting for user action"] = "Degraded",
    ["Primary instance is being restarted in-place"] = "Degraded",
    ["Primary instance is being restarted without a switchover"] = "Degraded",
    ["Cluster cannot execute instance online upgrade due to missing architecture binary"] = "Degraded",
    ["Online upgrade in progress"] = "Degraded",
    ["Applying configuration"] = "Degraded",
    ["Unable to create required cluster objects"] = "Suspended",
    ["Cluster cannot proceed to reconciliation due to an unknown plugin being required"] = "Suspended",
    ["Cluster has incomplete or invalid image catalog"] = "Suspended",
    ["Cluster is unrecoverable and needs manual intervention"] = "Suspended",
}

function hibernating(obj)
    for i, condition in pairs(obj.status.conditions) do
        if condition.type == "cnpg.io/hibernation" then
            return condition
        end
    end
    return nil
end

if obj.status ~= nil and obj.status.conditions ~= nil then
    local hibernation = hibernating(obj)
    if hibernation ~= nil then
        if hibernation.status == "True" then
            hs.status = "Suspended"
            hs.message = hibernation.message
            return hs
        else
            hs.status = "Degraded"
            hs.message = hibernation.message
            return hs
        end
    end
    statusState = cnpgStatus[obj.status.phase]
    if statusState ~= nil then
        hs.status = statusState
        hs.message = obj.status.phaseReason
        return hs
    else
        hs.status = "Unknown"
        hs.message = obj.status.phaseReason
        return hs
    end
end

hs.status = "Progressing"
hs.message = obj.status.phaseReason
return hs
