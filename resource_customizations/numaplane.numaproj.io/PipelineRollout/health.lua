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
end

if obj.metadata.generation == obj.status.observedGeneration then
  if pipelinePaused.status == "True" and (obj.metadata.generation == pipelinePaused.observedGeneration) then
    hs.status = "Suspended"
    hs.message = "Pipeline is paused"
    return hs
  elseif healthyCondition.status == "True" and (obj.metadata.generation == healthyCondition.observedGeneration) then
    hs.status = "Healthy"
    hs.message = healthyCondition.message
    return hs
  elseif (healthyCondition.status == "False" and (obj.metadata.generation == healthyCondition.observedGeneration)) or obj.status.phase == "Failed" then
    hs.status = "Degraded"
    hs.message = healthyCondition.message
    return hs
  end
end

hs.status = "Progressing"
hs.message = "Waiting for Pipeline status"
return hs