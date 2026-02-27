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
local isSuspended = false
if obj.metadata and obj.metadata.annotations and obj.metadata.annotations["cnpg.io/reconciliation"] == "disabled" then
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
