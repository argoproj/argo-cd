local actions = {}
local is_rolling = false
if obj.metadata.annotations ~= nil then
    if obj.metadata.annotations["strimzi.io/manual-rolling-update"] == "true" then
        is_rolling = true
    end
end
actions["rolling-update"] = {["disabled"] = is_rolling}
return actions
