local hs = {}
local healthy = {}

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.status == "False" then
        healthy.status = "False"
      end
    end
  end

  if obj.metadata.generation == obj.status.observedGeneration then
    if (healthy ~= {} and healthy.status == "False") or obj.status.phase == "Failed" then
      hs.status = "Degraded"
      if obj.status.phase == "Failed" then
        hs.message = obj.status.message
      else
        hs.message = "Subresources are unhealthy"
      end
      return hs
    elseif obj.status.phase == "Paused" or obj.status.phase == "Pausing" then
      hs.status = "Suspended"
      hs.message = "Pipeline is paused"
      return hs
    elseif obj.status.phase == "Running" then
      hs.status = "Healthy"
      hs.message = "Pipeline is healthy"
      return hs
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for Pipeline status"
return hs