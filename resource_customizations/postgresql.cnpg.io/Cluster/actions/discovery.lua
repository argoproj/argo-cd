local actions = {}
actions["restart"] = {
    ["iconClass"] = "fa fa-fw fa-recycle",
    ["displayName"] = "Rollout restart Cluster"
}
actions["reload"] = {
    ["iconClass"] = "fa fa-fw fa-rotate-right",
    ["displayName"] = "Reload all Configuration"
}
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

-- Check if reconciliation is currently suspended
local isReconcileSuspended = false
if obj.metadata and obj.metadata.annotations and obj.metadata.annotations["cnpg.io/reconciliationLoop"] == "disabled" then
    isReconcileSuspended = true
end

-- Add reconcile suspend/resume actions based on current state
if isReconcileSuspended then
    actions["reconcile-resume"] = {
        ["iconClass"] = "fa fa-fw fa-play",
        ["displayName"] = "Reconcile Resume"
    }
else
    actions["reconcile-suspend"] = {
        ["iconClass"] = "fa fa-fw fa-pause",
        ["displayName"] = "Reconcile Suspend"
    }
end

-- Add suspend/resume actions based on current state
local isHibernated = false
if obj.metadata and obj.metadata.annotations and obj.metadata.annotations["cnpg.io/hibernation"] == "on" then
    isHibernated = true
end

if isHibernated then
    actions["hibernate"] = {
        ["iconClass"] = "fa fa-fw fa-pause",
        ["displayName"] = "Cluster Hibernate"
    }
end

if isHibernated then
    actions["rehydrate"] = {
        ["iconClass"] = "fa fa-fw fa-play",
        ["displayName"] = "Cluster Rehydrate"
    }
end

return actions
