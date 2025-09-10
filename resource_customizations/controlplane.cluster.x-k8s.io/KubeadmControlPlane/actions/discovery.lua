local actions = {}
actions["restart"] = {}

local paused = false
if obj.metadata.annotations ~= nil and obj.metadata.annotations["cluster.x-k8s.io/paused"] ~= nil then
    paused = true
end
actions["pause"] = {["disabled"] = paused}
actions["resume"] = {["disabled"] = not(paused)}
return actions
