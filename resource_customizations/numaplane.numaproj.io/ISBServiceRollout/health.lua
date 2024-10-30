local hs = {}
local healthyCondition = {}

-- check for certain cases of "Progressing"

if obj.status == nil then -- if there's no Status at all, we haven't been reconciled
  hs.status = "Progressing"
  hs.message = "Not yet reconciled"
  return hs
end

if obj.metadata.generation ~= obj.status.observedGeneration then
  hs.status = "Progressing"
  hs.message = "Not yet reconciled"
  return hs
end

if obj.status.phase == "Pending" then
  hs.status = "Progressing"
  hs.message = "Phase=Pending"
  return hs
end

if obj.status.upgradeInProgress ~= nil and obj.status.upgradeInProgress ~= "" then
  hs.status = "Progressing"
  hs.message = "Update in progress"
  return hs
end

-- now check the Conditions

if obj.status.conditions ~= nil then
  for i, condition in ipairs(obj.status.conditions) do
    if condition.type == "ChildResourcesHealthy" then
      healthyCondition = condition
    end
  end
end

if (healthyCondition ~= {} and healthyCondition.status == "False" and healthyCondition.reason == "ISBSvcFailed") or obj.status.phase == "Failed" then
  hs.status = "Degraded"
  if obj.status.phase == "Failed" then
    hs.message = obj.status.message
  else
    hs.message = healthyCondition.message
  end
  return hs
elseif healthyCondition ~= {} and healthyCondition.status == "False" and healthyCondition.reason == "Progressing" then
  hs.status = "Progressing"
  hs.message = healthyCondition.message
  return hs
elseif healthyCondition ~= {} and healthyCondition.status == "True" and obj.status.phase == "Deployed" then
  hs.status = "Healthy"
  hs.message = healthyCondition.message
  return hs
end

hs.status = "Unknown"
hs.message = "Unknown ISBService status"
return hs