function getCurrentCanaryStep(obj)

    if obj.spec.strategy.canary == nil or table.getn(obj.spec.strategy.canary.steps) == 0 then
        return nil
    end

	local currentStepIndex = 0

	if obj.status.currentStepIndex ~= nil then
		currentStepIndex = obj.status.currentStepIndex
    end

	return currentStepIndex

end

if obj.spec.paused ~= nil and obj.spec.paused then
    obj.spec.paused = false
end

local isInconclusive = obj.spec.strategy.canary ~= nil and obj.status.canary.currentStepAnalysisRunStatus ~= nil and obj.status.canary.currentStepAnalysisRunStatus.status == "Inconclusive"

if isInconclusive and obj.status.pauseConditions ~= nil and table.getn(obj.status.pauseConditions) > 0 and obj.status.controllerPause ~= nil and obj.status.controllerPause then

    local currentStepIndex = getCurrentCanaryStep(obj)
    if currentStepIndex ~= nil then
        if currentStepIndex < table.getn(obj.spec.strategy.canary.steps) then
            currentStepIndex = currentStepIndex + 1
        end

        obj.status.pauseConditions = nil
        obj.status.controllerPause = false
        obj.status.currentStepIndex = currentStepIndex
    end

elseif obj.status.pauseConditions ~= nil and table.getn(obj.status.pauseConditions) > 0 then
    
    obj.status.pauseConditions = nil

elseif obj.spec.strategy.canary ~= nil then
    
    local currentStepIndex = getCurrentCanaryStep(obj)
    if currentStepIndex ~= nil then
        if currentStepIndex < table.getn(obj.spec.strategy.canary.steps) then
            currentStepIndex = currentStepIndex + 1
        end

        obj.status.pauseConditions = nil
        obj.status.currentStepIndex = currentStepIndex
    end

end

return obj