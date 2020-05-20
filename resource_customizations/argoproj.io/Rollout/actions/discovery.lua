actions = {}
actions["resume"] = {["disabled"] = false}
actions["restart"] = {["disabled"] = false}

local paused = false

if obj.status ~= nil and obj.status.pauseConditions ~= nil then
    paused = table.getn(obj.status.pauseConditions) > 0
elseif obj.spec.paused ~= nil then
    paused = obj.spec.paused
end
if paused then
    actions["resume"]["disabled"] = false
else
    actions["resume"]["disabled"] = true
end

return actions
