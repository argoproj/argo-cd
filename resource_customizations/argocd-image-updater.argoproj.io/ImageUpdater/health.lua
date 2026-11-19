local hs = {}

-- No status yet: controller hasn't reconciled
if obj.status == nil or obj.status.conditions == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for ImageUpdater to be reconciled"
  return hs
end

-- observedGeneration guard: spec change not yet picked up by controller
if obj.metadata ~= nil and obj.metadata.generation ~= nil and obj.status.observedGeneration ~= nil
  and obj.status.observedGeneration ~= obj.metadata.generation then
    hs.status = "Progressing"
    hs.message = "Waiting for ImageUpdater spec update to be observed"
    return hs
end

-- Walk conditions in priority order: Error > Reconciling > Ready
for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "Error" and condition.status == "True" then
    hs.status = "Degraded"
    hs.message = condition.message
    return hs
  end
end

for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "Reconciling" and condition.status == "True" then
    hs.status = "Progressing"
    hs.message = condition.message
    return hs
  end
end

for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "Ready" and condition.status == "True" then
    hs.status = "Healthy"
    hs.message = condition.message
    return hs
  end
  if condition.type == "Ready" and condition.status == "False" then
    hs.status = "Degraded"
    hs.message = condition.message
    return hs
  end
end

hs.status = "Progressing"
hs.message = "Waiting for ImageUpdater to report a Ready condition"
return hs
