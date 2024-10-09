local hs = {}
local healthyCondition = {}

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "ChildrenResourcesHealthy" then
        healthyCondition = condition
      end
    end
  end

  if obj.metadata.generation == obj.status.observedGeneration then
    if (healthyCondition ~= {} and healthyCondition.status == "False") or obj.status.phase == "Failed" then
      hs.status = "Degraded"
      if obj.status.phase == "Failed" then
        hs.message = obj.status.message
      else
        hs.message = healthyCondition.message
      end
      return hs
    elseif (healthyCondition ~= {} and healthyCondition.status == "True") and obj.status.phase == "Running" then
      hs.status = "Healthy"
      hs.message = healthyCondition.message
      return hs
    end
  end   
end

hs.status = "Progressing"
hs.message = "Waiting for InterStepBufferService status"
return hs