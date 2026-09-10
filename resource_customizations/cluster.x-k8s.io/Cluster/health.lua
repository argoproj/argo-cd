function getStatusBasedOnPhase(obj, hs)
    hs.status = "Progressing"
    hs.message = "Waiting for clusters"
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

function getReadyContitionStatus(obj, hs)
    if obj.status ~= nil and obj.status.conditions ~= nil then
        for i, condition in ipairs(obj.status.conditions) do
        -- Ready summarizes ControlPlaneReady + InfrastructureReady and is False with
        -- severity Info for the entire normal provisioning window.
        -- Severity is mirrored/summarized verbatim from the underlying provider
        -- condition, so a real failure still reports severity Error here regardless
        -- of phase - only suppress the Info-severity "still working" case.
        if condition.type == "Ready" and condition.status == "False" and condition.severity ~= "Info" then
            hs.status = "Degraded"
            hs.message = condition.message
            return hs
        end
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

getStatusBasedOnPhase(obj, hs)
getReadyContitionStatus(obj, hs)

return hs
