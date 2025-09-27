-- return true if degraded, along with the reason
function isDegraded(obj) 
  if obj.status == nil then 
    return false, ""
  end
  -- check phase=Failed, healthy condition failed, progressive upgrade failed
  if obj.status.phase == "Failed" then
    return true, obj.status.message
  end

  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "ChildResourcesHealthy" and condition.status == "False" and condition.reason == "ISBSvcFailed" then
        return true, condition.message
      elseif condition.type == "ProgressiveUpgradeSucceeded" and condition.status == "False" then
        return true, "Progressive upgrade failed"
      end
    end
  end

  return false, ""
end

function isProgressing(obj) 
  -- if there's no Status at all, we haven't been reconciled
  if obj.status == nil then 
    return true, "Not yet reconciled"
  end

  if obj.metadata.generation ~= obj.status.observedGeneration then
    return true, "Not yet reconciled"
  end

  -- if we are in the middle of an upgrade
  if obj.status.upgradeInProgress ~= nil and obj.status.upgradeInProgress ~= "" or obj.status.phase == "Pending" then
    -- first check if Progressive Upgrade Failed; in that case, we won't return true (because "Degraded" will take precedence)
    progressiveUpgradeFailed = false
    if obj.status.conditions ~= nil then
      for i, condition in ipairs(obj.status.conditions) do
        if condition.type == "ProgressiveUpgradeSucceeded" and condition.status == "False" then
          progressiveUpgradeFailed = true
        end
      end
    end

    if progressiveUpgradeFailed == false then
      return true, "Update in progress"
    end
  end

  -- if the child is Progressing
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "ChildResourcesHealthy" and condition.status == "False" and condition.reason == "Progressing" then
        return true, "Child Progressing"
      end
    end
  end

  return false, ""
end

-- return true if healthy, along with the reason
function isHealthy(obj)
  if obj.status == nil then 
    return false, ""
  end

  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "ChildResourcesHealthy" and condition.status == "True" then
        return true, "Healthy"
      end
    end
  end
end

local hs = {}


progressing, reason = isProgressing(obj)
if progressing then
  hs.status = "Progressing"
  hs.message = reason
  return hs
end

degraded, reason = isDegraded(obj)
if degraded then
  hs.status = "Degraded"
  hs.message = reason
  return hs
end

healthy, reason = isHealthy(obj)
if healthy then
  hs.status = "Healthy"
  hs.message = reason
  return hs
end

hs.status = "Unknown"
hs.message = "Unknown status"
return hs
