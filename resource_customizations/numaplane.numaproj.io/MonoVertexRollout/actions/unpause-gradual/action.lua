obj.spec.monoVertex.spec.lifecycle.desiredPhase = "Running"
obj.spec.monoVertex.spec.replicas = null
-- set strategy.fastResume to false
if obj.spec.strategy == nil then
    obj.spec.strategy = {}
end
if obj.spec.strategy.pauseResume == nil then
    obj.spec.strategy.pauseResume = {}
end
obj.spec.strategy.pauseResume.fastResume = false
return obj