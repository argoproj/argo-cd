function checkReplicasStatus(obj)
  hs = {}
  replicasCount = getNumberValueOrDefault(obj.spec.replicas)
  replicasStatus = getNumberValueOrDefault(obj.status.replicas)
  updatedReplicas = getNumberValueOrDefault(obj.status.updatedReplicas)
  availableReplicas = getNumberValueOrDefault(obj.status.availableReplicas)

  if updatedReplicas < replicasCount then
    hs.status = "Progressing"
    hs.message = "Waiting for roll out to finish: More replicas need to be updated"
    return hs
  end
  -- Since the scale down delay can be very high, BlueGreen does not wait for all the old replicas to scale
  -- down before marking itself healthy. As a result, only evaluate this condition if the strategy is canary.
  if obj.spec.strategy.canary ~= nil and replicasStatus > updatedReplicas then
    hs.status = "Progressing"
    hs.message = "Waiting for roll out to finish: old replicas are pending termination"
    return hs
  end
  if availableReplicas < updatedReplicas then
    hs.status = "Progressing"
    hs.message = "Waiting for roll out to finish: updated replicas are still becoming available"
    return hs
  end
  return nil
end

-- In Argo Rollouts v0.8 we depreciate .status.canary.stableRS for .status.stableRS this func grabs the correct one
function getStableRS(obj)
    if obj.status.stableRS ~= nil then
        return obj.status.stableRS
    end
    if obj.status.canary ~= nil then
            return obj.status.canary.stableRS
    end
    return ""
end

function getNumberValueOrDefault(field)
  if field ~= nil then
    return field
  end
  return 0
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

hs = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
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
  end
  if obj.status.currentPodHash ~= nil then
    if obj.spec.strategy.blueGreen ~= nil then
      isPaused = checkPaused(obj)
      if isPaused ~= nil then
        return isPaused
      end
      replicasHS = checkReplicasStatus(obj)
      if replicasHS ~= nil then
        return replicasHS
      end
      if obj.status.blueGreen ~= nil and obj.status.blueGreen.activeSelector ~= nil and obj.status.blueGreen.activeSelector == obj.status.currentPodHash then
        hs.status = "Healthy"
        hs.message = "The active Service is serving traffic to the current pod spec"
        return hs
      end
      hs.status = "Progressing"
      hs.message = "The current pod spec is not receiving traffic from the active service"
      return hs
    end
    if obj.spec.strategy.canary ~= nil then
      currentRSIsStable = getStableRS(obj) == obj.status.currentPodHash
      if obj.spec.strategy.canary.steps ~= nil and table.getn(obj.spec.strategy.canary.steps) > 0 then
        stepCount = table.getn(obj.spec.strategy.canary.steps)
        if obj.status.currentStepIndex ~= nil then
          currentStepIndex = obj.status.currentStepIndex
          isPaused = checkPaused(obj)
          if isPaused ~= nil then
            return isPaused
          end
      
          if paused then
            hs.status = "Suspended"
            hs.message = "Rollout is paused"
            return hs
          end
          if currentRSIsStable and stepCount == currentStepIndex then
            replicasHS = checkReplicasStatus(obj)
            if replicasHS ~= nil then
              return replicasHS
            end
            hs.status = "Healthy"
            hs.message = "The rollout has completed all steps"
            return hs
          end
        end
        hs.status = "Progressing"
        hs.message = "Waiting for rollout to finish steps"
        return hs
      end

      -- The detecting the health of the Canary deployment when there are no steps
      replicasHS = checkReplicasStatus(obj)
      if replicasHS ~= nil then
        return replicasHS
      end
      if currentRSIsStable then
        hs.status = "Healthy"
        hs.message = "The rollout has completed canary deployment"
        return hs
      end
      hs.status = "Progressing"
      hs.message = "Waiting for rollout to finish canary deployment"
    end
  end
end
hs.status = "Progressing"
hs.message = "Waiting for rollout to finish: status has not been reconciled."
return hs