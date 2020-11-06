obj.status.abort = nil
if obj.spec.strategy.canary.steps ~= nil then
    obj.status.currentStepIndex = table.getn(obj.spec.strategy.canary.steps)
end
return obj
