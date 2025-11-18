local actions = {}
local paused = false
local hasPausedReplicas = false
local currentPausedReplicas = nil
 
if obj.metadata and obj.metadata.annotations then
    paused = obj.metadata.annotations["autoscaling.keda.sh/paused"] == "true"
    currentPausedReplicas = obj.metadata.annotations["autoscaling.keda.sh/paused-replicas"]
    hasPausedReplicas = currentPausedReplicas ~= nil
end
 
local isPaused = paused or hasPausedReplicas
 
actions["pause"] = {
    ["disabled"] = isPaused,
    ["iconClass"] = "fa fa-fw fa-pause-circle"
}
 
actions["paused-replicas"] = {
    ["disabled"] = paused,
    ["iconClass"] = "fa fa-fw fa-pause-circle",
    ["params"] = {
        {
            ["name"] = "replicas",
            ["default"] = currentPausedReplicas or "0"
        }
    },
}
 
actions["resume"] = {
    ["disabled"] = not isPaused,
    ["iconClass"] = "fa fa-fw fa-play-circle"
}
 
return actions
