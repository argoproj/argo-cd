function checkReplicasStatus(obj)
  hs = {}
  desiredReplicas = getNumberValueOrDefault(obj.spec.replicas, 1)
  statusReplicas = getNumberValueOrDefault(obj.status.replicas, 0)
  updatedReplicas = getNumberValueOrDefault(obj.status.updatedReplicas, 0)
  availableReplicas = getNumberValueOrDefault(obj.status.availableReplicas, 0)
  
  if updatedReplicas < desiredReplicas then
    hs.status = "Progressing"
    hs.message = "Waiting for roll out to finish: More replicas need to be updated"
    return hs
  end
  if availableReplicas < updatedReplicas then
    hs.status = "Progressing"
    hs.message = "Waiting for roll out to finish: updated replicas are still becoming available"
    return hs
  end
  return nil
end

-- In Argo Rollouts v0.8 we deprecated .status.canary.stableRS for .status.stableRS
-- This func grabs the correct one.
function getStableRS(obj)
  if obj.status.stableRS ~= nil then
    return obj.status.stableRS
  end
  if obj.status.canary ~= nil then
      return obj.status.canary.stableRS
  end
  return ""
end

function getNumberValueOrDefault(field, default)
  if field ~= nil then
    return field
  end
  return default
end

function checkPaused(obj)
  hs = {}
  hs.status = "Suspended"
  hs.message = "Rollout is paused"
  if obj.status.pauseConditions ~= nil and table.getn(obj.status.pauseConditions) > 0 then
    return hs
  end

  if obj.spec.paused ~= nil and obj.spec.paused then
    return hs
  end
  return nil
end

-- isGenerationObserved determines if the rollout spec has been observed by the controller. This
-- only applies to v0.10 rollout which uses a numeric status.observedGeneration. For v0.9 rollouts
-- and below this function always returns true.
function isGenerationObserved(obj)
  if obj.status == nil then
    return false
  end
  observedGeneration = tonumber(obj.status.observedGeneration)
  if observedGeneration == nil or observedGeneration > obj.metadata.generation then
    -- if we get here, the rollout is a v0.9 rollout
    return true
  end
  return observedGeneration == obj.metadata.generation
end

hs = {}
if not isGenerationObserved(obj) then
  hs.status = "Progressing"
  hs.message = "Waiting for rollout spec update to be observed"
  return hs
end  

for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "InvalidSpec" then
    hs.status = "Degraded"
    hs.message = condition.message
    return hs
  end
  if condition.type == "Progressing" and condition.reason == "RolloutAborted" then
    hs.status = "Degraded"
    hs.message = condition.message
    return hs
  end
  if condition.type == "Progressing" and condition.reason == "ProgressDeadlineExceeded" then
    hs.status = "Degraded"
    hs.message = condition.message
    return hs
  end
end

isPaused = checkPaused(obj)
if isPaused ~= nil then
  return isPaused
end

if obj.status.currentPodHash == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for rollout to finish: status has not been reconciled."
  return hs
end

replicasHS = checkReplicasStatus(obj)
if replicasHS ~= nil then
  return replicasHS
end


stableRS = getStableRS(obj)

if obj.spec.strategy.blueGreen ~= nil then
  if obj.status.blueGreen == nil or obj.status.blueGreen.activeSelector ~= obj.status.currentPodHash then
    hs.status = "Progressing"
    hs.message = "active service cutover pending"
    return hs
  end
  -- Starting in v0.8 blue-green uses status.stableRS. To drop support for v0.7, uncomment following
  -- if stableRS == "" or stableRS ~= obj.status.currentPodHash then
  if stableRS ~= "" and stableRS ~= obj.status.currentPodHash then
    hs.status = "Progressing"
    hs.message = "waiting for analysis to complete"
    return hs
  end
elseif obj.spec.strategy.canary ~= nil then
  if statusReplicas > updatedReplicas then
    hs.status = "Progressing"
    hs.message = "Waiting for roll out to finish: old replicas are pending termination"
    return hs
  end
  if stableRS == "" or stableRS ~= obj.status.currentPodHash then
    hs.status = "Progressing"
    hs.message = "Waiting for rollout to finish steps"
    return hs
  end
end

hs.status = "Healthy"
hs.message = ""
return hs