local actions = {}
actions["restart"] = {}

local paused = false
if obj.spec.paused ~= nil then
    paused = obj.spec.paused
    actions["pause"] = {paused}
end
actions["resume"] = {["disabled"] = not(paused)}

actions["scale"] = {
    ["params"] = {
        {
            ["name"] = "replicas",
            ["default"] = tostring(obj.spec.replicas)
        }
    },
}
return actions
