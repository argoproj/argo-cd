if obj.status == nil or not obj.status.abort then
    return obj
end

obj.status.abort = nil
if obj.status.blueGreen ~= nil then
    obj.status.blueGreen.prePromotionAnalysisRunStatus = nil
    obj.status.blueGreen.postPromotionAnalysisRunStatus = nil
end
if obj.status.canary ~= nil then
    obj.status.canary.currentStepAnalysisRunStatus = nil
    obj.status.canary.currentBackgroundAnalysisRunStatus = nil
end
return obj
