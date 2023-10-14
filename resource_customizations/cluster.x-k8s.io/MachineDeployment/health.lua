local hs = {}
hs.status = "Progressing"
hs.message = "Waiting for machines"

if obj.spec.paused ~= nil and obj.spec.paused then
    hs.status = "Suspended"
    hs.message = "MachineDeployment is paused"
    return hs
end

if obj.status ~= nil and obj.status.phase ~= nil then
    if obj.status.phase == "Running" then
        hs.status = "Healthy"
        hs.message = "Machines are running under this deployment"
    end
    if obj.status.phase == "ScalingUp" then
        hs.status = "Progressing"
        hs.message = "Cluster is spawning machines"
    end
    if obj.status.phase == "ScalingDown" then
        hs.status = "Progressing"
        hs.message = "Cluster is stopping machines"
    end
    if obj.status.phase == "Failed" then
        hs.status = "Degraded"
        hs.message = "MachineDeployment is failed"
    end
end

return hs