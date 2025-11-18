local actions = {}

actions["suspend"] = {["disabled"] = true}
actions["resume"] = {["disabled"] = true}

local suspend = false
if obj.spec.suspend ~= nil then
    suspend = obj.spec.suspend
end
if suspend then
    actions["resume"]["disabled"] = false
else
    actions["suspend"]["disabled"] = false
end

return actions
