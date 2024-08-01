local hs = {}
local healthyCondition = {}

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "ChildResourcesHealthy" then
        healthyCondition = condition
      end
    end
  end

  if obj.metadata.generation == obj.status.observedGeneration then
    if (healthyCondition ~= {} and healthyCondition.status == "False" and (obj.metadata.generation == healthyCondition.observedGeneration) and healthyCondition.reason == "Degraded") or obj.status.phase == "Failed" then
      hs.status = "Degraded"
      if obj.status.phase == "Failed" then
        hs.message = obj.status.message
      else
        hs.message = healthyCondition.message
      end
      return hs
    elseif healthyCondition ~= {} and healthyCondition.status == "True" and (obj.metadata.generation == healthyCondition.observedGeneration) and obj.status.phase == "Deployed" then
      hs.status = "Healthy"
      hs.message = healthyCondition.message
      return hs
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for NumaflowController status"
return hs