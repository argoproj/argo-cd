if obj.status.pauseConditions ~= nil and table.getn(obj.status.pauseConditions) > 0 then
    obj.status.pauseConditions = nil
end

if obj.spec.paused ~= nil and obj.spec.paused then
    obj.spec.paused = false
end

return obj