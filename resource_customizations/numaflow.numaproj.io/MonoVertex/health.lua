local hs = {}
local podsHealth = {}
local daemonHealth = {}

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "PodsHealthy" then
        podsHealth = condition
      end
      if condition.type == "DaemonHealthy" then
        daemonHealth = condition
      end
    end
  end

  if obj.metadata.generation == obj.status.observedGeneration then
    if (podsHealth ~= {} and podsHealth.status == "False") or (daemonHealth ~= {} and daemonHealth.status == "False") or obj.status.phase == "Failed" then
      hs.status = "Degraded"
      if obj.status.phase == "Failed" then
        hs.message = obj.status.message
      else
        hs.message = "Subresources are unhealthy"
      end
      return hs
    elseif obj.status.phase == "Paused" then
      hs.status = "Suspended"
      hs.message = "MonoVertex is paused"
      return hs
    elseif (podsHealth ~= {} and podsHealth.status == "True") and (daemonHealth ~= {} and daemonHealth.status == "True") and obj.status.phase == "Running" then
      hs.status = "Healthy"
      hs.message = "MonoVertex is healthy"
      return hs
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for MonoVertex status"
return hs
