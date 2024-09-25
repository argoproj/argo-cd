local actions = {}
actions["restart"] = {}

local paused = false
if obj.spec.paused ~= nil then
    paused = obj.spec.paused
    actions["pause"] = {paused}
end
actions["resume"] = {["disabled"] = not(paused)}
actions["scale"] = {["defaultValue"] = tostring(obj.spec.replicas), ["hasParameters"] = true, ["errorMessage"] = "Enter any valid number", ["regexp"] = "^[0-9]*$", }
return actions
