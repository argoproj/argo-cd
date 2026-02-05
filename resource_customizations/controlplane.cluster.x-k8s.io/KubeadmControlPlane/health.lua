-- Helper to find specific condition by type
local function getCondition(conditions, condType)
    if not conditions then return nil end
    for _, c in ipairs(conditions) do
        if c.type == condType then
            return c
        end
    end
    return nil
end

-- Initialize health status as Progressing
local hs = {}
hs.status = "Progressing"
hs.message = "Waiting for KubeadmControlPlane status"

-- If .status is nil, the control plane not ready
if obj.status == nil then
    return hs
end

-- Extract v1beta2 and legacy condition arrays (CAPI supports both v1beta2 & legacy conditions)
local v1beta2 = obj.status.v1beta2 and obj.status.v1beta2.conditions
local legacy = obj.status.conditions

-- Check etcd health first. If unhealthy control plane is degraded
local etcdCond = getCondition(legacy, "EtcdClusterHealthy")
if etcdCond and etcdCond.status ~= "True" then
    hs.status = "Degraded"
    hs.message = "Etcd cluster is unhealthy"
    return hs
end

-- Check critical conditions that indicate control plane is still progressing
local checks = {
    {conditions = v1beta2, type = "MachinesReady", message = "Machines not ready"},
    {conditions = v1beta2, type = "MachinesUpToDate", message = "Updating machines"},
    {conditions = legacy, type = "ControlPlaneComponentsHealthy", message = "Control plane components unhealthy"},
}

-- Iterate over critical conditions to determine if control plane is progressing
for _, check in ipairs(checks) do
    local cond = getCondition(check.conditions, check.type)
    if cond and cond.status ~= "True" then
        hs.status = "Progressing"
        hs.message = check.message
        return hs
    end
end

-- Check if a rolling update in progress
local rolloutCond = getCondition(v1beta2, "RollingOut")
if rolloutCond and rolloutCond.status == "True" then
    hs.status = "Progressing"
    hs.message = "Rolling out"
    return hs
end

-- Extract replica counts to check update and readiness progress
local replicas = obj.status.replicas or 0
local updated = obj.status.updatedReplicas or 0
local ready = obj.status.readyReplicas or 0

-- If not all replicas updated, control plane progressing
if replicas > 0 and updated < replicas then
    hs.status = "Progressing"
    hs.message = string.format("Updating: %d/%d updated, %d ready", updated, replicas, ready)
    return hs
end

-- If not all replicas ready, control plane progressing
if replicas ~= ready then
    hs.status = "Progressing"
    hs.message = string.format("%d/%d ready", ready, replicas)
    return hs
end

-- At this point all checks passed & control plane is healthy
hs.status = "Healthy"
hs.message = "Control plane is healthy"
return hs
