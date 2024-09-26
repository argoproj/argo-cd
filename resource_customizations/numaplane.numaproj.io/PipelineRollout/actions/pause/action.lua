if obj.spec.pipeline.spec.lifecycle == nil then
  obj.spec.pipeline.spec.lifecycle = {}
end
obj.spec.pipeline.spec.lifecycle.desiredPhase = "Paused"
return obj