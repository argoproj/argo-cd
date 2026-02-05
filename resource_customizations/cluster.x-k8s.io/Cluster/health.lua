function getStatusBasedOnConditions(obj, hs)
    conditions = nil
    if obj.status ~= nil and obj.status.conditions ~= nil then
        conditions = obj.status.conditions
    end
    if obj.status ~= nil and obj.status.v1beta2 ~= nil and obj.status.v1beta2.conditions ~= nil then
        conditions = obj.status.v1beta2.conditions
    end
    if conditions == nil then
        return
    end
    for i, condition in ipairs(conditions) do
        -- Mandatory conditions, leading to Degraded if not True
        if
            (
                condition.type == "Available"
                or condition.type == "Ready"
                or condition.type == "RemoteConnectionProbe"
                or condition.type == "InfrastructureReady"
                or condition.type == "ControlPlaneInitialized"
                or condition.type == "ControlPlaneAvailable"
                or condition.type == "WorkersAvailable"
                or condition.type == "ControlPlaneMachinesReady"
            )
            and condition.status ~= "True"
        then
            hs.status = "Degraded"
            hs.message = condition.message
            return
        end
        -- Transcient conditions, leading to Progressing if not False
        if
            (
                condition.type == "RollingOut"
                or condition.type == "Remediating"
                or condition.type == "ScalingDown"
                or condition.type == "ScalingUp"
                or condition.type == "Deleting"
            )
            and condition.status ~= "False"
        then
            hs.status = "Progressing"
            hs.message = condition.message
            return
        end
        -- Transcient conditions, leading to Progressing if not True
        if
            (
                condition.type == "WorkerMachinesReady"
                or condition.type == "ControlPlaneMachinesUpToDate"
                or condition.type == "WorkerMachinesUpToDate"
                or condition.type == "TopologyReconciled"
            )
            and condition.status ~= "True"
        then
            hs.status = "Progressing"
            hs.message = condition.message
            return
        end
    end
end

function getStatusBasedOnPhase(obj, hs)
    if obj.status ~= nil and obj.status.phase ~= nil then
        if obj.status.phase == "Provisioned" then
            hs.status = "Healthy"
            hs.message = "Cluster is running"
        end
        if obj.status.phase == "Failed" then
            hs.status = "Degraded"
            hs.message = ""
        end
    end
    return hs
end

local hs = {}

if obj.spec.paused ~= nil and obj.spec.paused then
    hs.status = "Suspended"
    hs.message = "Cluster is paused"
    return hs
end

getStatusBasedOnConditions(obj, hs)
if hs.status == "Degraded" or hs.status == "Progressing" then
    return hs
end

getStatusBasedOnPhase(obj, hs)

if hs.status == nil then
    -- Default status
    hs.status = "Progressing"
    hs.message = "Waiting for Cluster"
end

return hs
