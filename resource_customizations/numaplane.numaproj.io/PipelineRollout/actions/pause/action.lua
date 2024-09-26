if obj.spec.monoVertex.spec.lifecycle == nil then
  obj.spec.monoVertex.spec.lifecycle = {}
end
obj.spec.pipeline.spec.lifecycle.desiredPhase = "Paused"
return obj