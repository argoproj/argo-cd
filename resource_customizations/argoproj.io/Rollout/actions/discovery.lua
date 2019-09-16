actions = {}
actions["resume"] = {["available"] = false}

local paused = false

if obj.status ~= nil and obj.status.verifyingPreview ~= nil then
    paused = obj.status.verifyingPreview
elseif obj.spec.paused ~= nil then
    paused = obj.spec.paused
end
if paused then
    actions["resume"]["available"] = true
end

return actions
