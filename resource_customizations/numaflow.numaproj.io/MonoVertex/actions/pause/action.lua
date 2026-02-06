if obj.spec.lifecycle == nil then
    obj.spec.lifecycle = {}
  end
  obj.spec.lifecycle.desiredPhase = "Paused"
  return obj