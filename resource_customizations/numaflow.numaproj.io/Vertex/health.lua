local hs = {}
local podsHealth = {}

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "PodsHealthy" then
        podsHealth = condition
      end
    end
  end

  if obj.metadata.generation == obj.status.observedGeneration then
    if (podsHealth ~= {} and podsHealth.status == "False") or obj.status.phase == "Failed" then
      hs.status = "Degraded"
      if obj.status.phase == "Failed" then
        hs.message = obj.status.message
      else
        hs.message = podsHealth.message
      end
      return hs
    elseif (podsHealth ~= {} and podsHealth.status == "True") and obj.status.phase == "Running" then
      hs.status = "Healthy"
      hs.message = podsHealth.message
      return hs
    end
  end
end
    
hs.status = "Progressing"
hs.message = "Waiting for Vertex status"
return hs