local hs = {}

if obj.status == nil or obj.status.conditions == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for KafkaMirrorMaker2"
  return hs
end

for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "ReconciliationPaused" and condition.status == "True" then
    hs.status = "Suspended"
    hs.message = "Reconciliation is paused"
    return hs
  end
end

if obj.metadata.generation ~= nil and obj.status.observedGeneration ~= nil and obj.status.observedGeneration < obj.metadata.generation then
  hs.status = "Progressing"
  hs.message = "Waiting for KafkaMirrorMaker2 spec update to be observed"
  return hs
end

local ready = false
for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "NotReady" and condition.status == "True" then
    hs.status = "Degraded"
    hs.message = condition.message or condition.reason or "KafkaMirrorMaker2 is not ready"
    return hs
  end
  if condition.type == "Ready" and condition.status == "True" then
    ready = true
  end
end

if not ready then
  hs.status = "Progressing"
  hs.message = "Waiting for KafkaMirrorMaker2"
  return hs
end

if obj.status.connectors ~= nil then
  for _, connector in ipairs(obj.status.connectors) do
    if connector.connector ~= nil and connector.connector.state == "FAILED" then
      hs.status = "Degraded"
      hs.message = "Connector " .. tostring(connector.name) .. " is FAILED"
      return hs
    end
    if connector.tasks ~= nil then
      for _, task in ipairs(connector.tasks) do
        if task.state == "FAILED" then
          hs.status = "Degraded"
          hs.message = "Task " .. tostring(task.id) .. " of connector " .. tostring(connector.name) .. " is FAILED"
          return hs
        end
      end
    end
  end
end

hs.status = "Healthy"
hs.message = ""
return hs
