actions = {}

if obj.spec.paused ~= nil and obj.spec.paused then
    actions["resume"] = {}
end

return actions
