local actions = {}
 
actions["restart"] = {
    ["iconClass"] = "fa fa-fw fa-redo"
}
 
local paused = false
if obj.spec.paused ~= nil then
    paused = obj.spec.paused
end

actions["pause"] = {
    ["disabled"] = paused,
    ["iconClass"] = "fa fa-fw fa-pause-circle"
}
 
actions["resume"] = {
    ["disabled"] = not(paused),
    ["iconClass"] = "fa fa-fw fa-play-circle"
}
 
actions["scale"] = {
    ["iconClass"] = "fa fa-fw fa-plus-circle",
    ["params"] = {
        {
            ["name"] = "replicas"
        }
    },
}
 
return actions