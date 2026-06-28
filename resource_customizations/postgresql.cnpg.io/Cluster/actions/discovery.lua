local actions = {}

-- https://github.com/cloudnative-pg/cloudnative-pg/tree/main/internal/cmd/plugin/restart
actions["restart"] = {
    ["iconClass"] = "fa fa-fw fa-recycle",
    ["displayName"] = "Rollout restart Cluster"
}

-- https://github.com/cloudnative-pg/cloudnative-pg/tree/main/internal/cmd/plugin/reload
actions["reload"] = {
    ["iconClass"] = "fa fa-fw fa-rotate-right",
    ["displayName"] = "Reload all Configuration"
}

-- https://github.com/cloudnative-pg/cloudnative-pg/tree/main/internal/cmd/plugin/promote
actions["promote"] = {
    ["iconClass"] = "fa fa-fw fa-angles-up",
    ["displayName"] = "Promote Replica to Primary",
    ["disabled"] = (not obj.status.instancesStatus or not obj.status.instancesStatus.healthy or #obj.status.instancesStatus.healthy < 2),
    ["params"] = {
        {
            ["name"] = "instance",
            ["default"] = "any"
        }
    }
}

-- Suspend reconciliation loop for a cluster
-- https://cloudnative-pg.io/docs/1.28/failure_modes/#disabling-reconciliation
local isSuspended = false
if obj.metadata and obj.metadata.annotations and obj.metadata.annotations["cnpg.io/reconciliationLoop"] == "disabled" then
    isSuspended = true
end

-- Add suspend/resume actions based on current state
if isSuspended then
    actions["resume"] = {
        ["iconClass"] = "fa fa-fw fa-play",
        ["displayName"] = "Resume Reconciliation"
    }
else
    actions["suspend"] = {
        ["iconClass"] = "fa fa-fw fa-pause",
        ["displayName"] = "Suspend Reconciliation"
    }
end

return actions
