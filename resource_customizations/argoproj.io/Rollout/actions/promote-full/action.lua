if obj.status ~= nil then
    generation = tonumber(obj.status.observedGeneration)
    if generation == nil or generation > obj.metadata.generation then
        -- rollouts v0.9 and below
        obj.status.abort = nil
        if obj.spec.strategy.canary.steps ~= nil then
            obj.status.currentStepIndex = table.getn(obj.spec.strategy.canary.steps)
        end
    else
        -- rollouts v0.10+
        obj.status.promoteFull = true
    end
end
return obj
