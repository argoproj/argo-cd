local hs = {}

if obj.status == nil or obj.status.conditions == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for KafkaMirrorMaker"
  return hs
end

if obj.metadata.generation ~= nil and obj.status.observedGeneration ~= nil and obj.status.observedGeneration < obj.metadata.generation then
  hs.status = "Progressing"
  hs.message = "Waiting for KafkaMirrorMaker spec update to be observed"
  return hs
end

for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "ReconciliationPaused" and condition.status == "True" then
    hs.status = "Suspended"
    hs.message = "Reconciliation is paused"
    return hs
  end
  if condition.type == "NotReady" and condition.status == "True" then
    hs.status = "Degraded"
    hs.message = condition.message or condition.reason or "KafkaMirrorMaker is not ready"
    return hs
  end
  if condition.type == "Ready" and condition.status == "True" then
    hs.status = "Healthy"
    hs.message = ""
    return hs
  end
end

hs.status = "Progressing"
hs.message = "Waiting for KafkaMirrorMaker"
return hs
