local hs = {}

if obj.metadata ~= nil and obj.metadata.labels ~= nil and obj.metadata.labels["numaplane.numaproj.io/progressive-result-state"] == "failed" then
  hs.status = "Degraded"
  hs.message = "Failed progressive upgrade"
  return hs
end

if obj.status == nil then
  hs.status = "Progressing"
  hs.message = "Not yet reconciled"
  return hs
end

if obj.metadata.generation ~= obj.status.observedGeneration then
  hs.status = "Progressing"
  hs.message = "Not yet reconciled"
  return hs
end

if obj.status.phase == "Failed" then
  hs.status = "Degraded"
  hs.message = obj.status.message
  return hs
end

if obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "ChildResourcesHealthy" then
      if condition.status == "True" then
        hs.status = "Healthy"
        hs.message = "Healthy"
        return hs
      end

      if condition.status == "False" then
        if condition.reason == "Progressing" then
          hs.status = "Progressing"
          hs.message = "Child Progressing"
          return hs
        end

        hs.status = "Degraded"
        hs.message = condition.message
        return hs
      end
    end
  end
end

hs.status = "Unknown"
hs.message = "Unknown status"
return hs
