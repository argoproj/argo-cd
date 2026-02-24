obj.spec.pipeline.spec.lifecycle.desiredPhase = "Running"
-- set strategy.fastResume to true
if obj.spec.strategy == nil then
    obj.spec.strategy = {}
end
if obj.spec.strategy.pauseResume == nil then
    obj.spec.strategy.pauseResume = {}
end
obj.spec.strategy.pauseResume.fastResume = true
return obj