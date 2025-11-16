local actions = {}

local paused = false
if obj.metadata and obj.metadata.annotations then
    paused = obj.metadata.annotations["autoscaling.keda.sh/paused"] == "true"
end

actions["pause"] = {["disabled"] = paused}
actions["resume"] = {["disabled"] = not paused}

return actions
