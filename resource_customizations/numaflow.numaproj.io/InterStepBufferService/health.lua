local hs = {}
local healthy = {}

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.status == "False" then
        healthy.status = "False"
        healthy.message = condition.message
      end
    end
  end

  progressiveFailure = (obj.metadata.labels ~= nil and obj.metadata.labels["numaplane.numaproj.io/progressive-result-state"] == "failed")
  if obj.metadata.generation == obj.status.observedGeneration then
    if (healthy ~= {} and healthy.status == "False") or obj.status.phase == "Failed" or progressiveFailure then
      hs.status = "Degraded"
      if obj.status.phase == "Failed" then
        hs.message = obj.status.message
      elseif progressiveFailure then
        hs.message = "Failed progressive upgrade"
      else
        hs.message = healthy.message
      end
      return hs
    elseif obj.status.phase == "Running" then
      hs.status = "Healthy"
      hs.message = "InterStepBufferService is healthy"
      return hs
    end
  end   
end

hs.status = "Progressing"
hs.message = "Waiting for InterStepBufferService status"
return hs