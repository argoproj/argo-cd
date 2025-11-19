local actions = {}

actions["restart"] = {}

local paused = false
if obj.spec.paused ~= nil then
    paused = obj.spec.paused
end

actions["pause"] = {
    ["disabled"] = paused,
}

actions["resume"] = {["disabled"] = not(paused)}

actions["scale"] = {
    ["params"] = {
        {
            ["name"] = "replicas"
        }
    },
}

return actions
