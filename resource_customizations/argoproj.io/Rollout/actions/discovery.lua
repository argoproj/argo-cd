actions = {}

local paused = false
if obj.status.verifyingPreview ~= nil then
    paused = obj.status.verifyingPreview
elseif obj.spec.paused ~= nil then
    paused = obj.spec.paused
end

if paused then
    actions["resume"] = {}
end

return actions
