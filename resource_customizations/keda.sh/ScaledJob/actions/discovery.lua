local actions = {}
local paused = false

if obj.metadata and obj.metadata.annotations then
    paused = obj.metadata.annotations["autoscaling.keda.sh/paused"] == "true"
end

actions["pause"] = {
    ["disabled"] = paused,
    ["iconClass"] = "fa fa-fw fa-pause-circle"
}

actions["resume"] = {
    ["disabled"] = not paused,
    ["iconClass"] = "fa fa-fw fa-play-circle"
}

return actions
