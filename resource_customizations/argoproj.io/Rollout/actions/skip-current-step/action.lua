if obj.status ~= nil then
    if obj.spec.strategy.canary ~= nil and obj.spec.strategy.canary.steps ~= nil and obj.status.currentStepIndex < table.getn(obj.spec.strategy.canary.steps) then
        if obj.status.pauseConditions ~= nil and table.getn(obj.status.pauseConditions) > 0 then
            obj.status.pauseConditions = nil
        end
        obj.status.currentStepIndex = obj.status.currentStepIndex + 1
    end
end
return obj