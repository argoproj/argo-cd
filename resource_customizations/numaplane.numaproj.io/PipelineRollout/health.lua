local hs = {}
local healthyCondition = {}
local pipelinePaused = {}

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "ChildResourcesHealthy" then
        healthyCondition = condition
      end
      if condition.type == "PipelinePausingOrPaused" then
        pipelinePaused = condition
      end
    end
  end

  if obj.metadata.generation == obj.status.observedGeneration then
    if (healthyCondition ~= {} and healthyCondition.status == "False" and (obj.metadata.generation == healthyCondition.observedGeneration) and healthyCondition.reason == "PipelineFailed") or obj.status.phase == "Failed" then
      hs.status = "Degraded"
      if obj.status.phase == "Failed" then
        hs.message = obj.status.message
      else
        hs.message = healthyCondition.message
      end
      return hs
    elseif (pipelinePaused ~= {} and pipelinePaused.status == "True") and (obj.metadata.generation == pipelinePaused.observedGeneration) then
      hs.status = "Suspended"
      hs.message = pipelinePaused.message
      return hs
    elseif (healthyCondition ~= {} and healthyCondition.status == "True") and (obj.metadata.generation == healthyCondition.observedGeneration) and obj.status.phase == "Deployed" then
      hs.status = "Healthy"
      hs.message = healthyCondition.message
      return hs
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for Pipeline status"
return hs