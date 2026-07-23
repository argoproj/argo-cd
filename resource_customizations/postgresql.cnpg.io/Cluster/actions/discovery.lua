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

-- Declarative hibernation for a cluster
-- https://cloudnative-pg.io/docs/current/declarative_hibernation/
local isHibernated = false
if obj.metadata and obj.metadata.annotations and obj.metadata.annotations["cnpg.io/hibernation"] == "on" then
    isHibernated = true
end

-- Add hibernate/rehydrate actions based on current state
if isHibernated then
    actions["rehydrate"] = {
        ["iconClass"] = "fa fa-fw fa-sun-o",
        ["displayName"] = "Rehydrate (exit hibernation)"
    }
else
    actions["hibernate"] = {
        ["iconClass"] = "fa fa-fw fa-moon-o",
        ["displayName"] = "Hibernate Cluster"
    }
end

return actions
