local hs = {}

local function num(val)
  if val == nil then
    return 0
  end
  return val
end

local function getDeploymentCondition(status, condType)
  if status.conditions == nil then
    return nil
  end
  for _, c in ipairs(status.conditions) do
    if c.type == condType then
      return c
    end
  end
  return nil
end

if obj.spec ~= nil and obj.spec.paused == true then
  hs.status = "Suspended"
  hs.message = "Deployment is paused"
  return hs
end

local generation = num(obj.metadata and obj.metadata.generation)
local status = obj.status or {}
local observedGeneration = num(status.observedGeneration)

if generation <= observedGeneration then
  local cond = getDeploymentCondition(status, "Progressing")
  if cond ~= nil and cond.reason == "ProgressDeadlineExceeded" then
    hs.status = "Degraded"
    hs.message = string.format('Deployment %q exceeded its progress deadline', obj.metadata.name)
    return hs
  end
  local specReplicas = obj.spec and obj.spec.replicas
  local updatedReplicas = num(status.updatedReplicas)
  local replicas = num(status.replicas)
  local availableReplicas = num(status.availableReplicas)
  if specReplicas ~= nil and updatedReplicas < specReplicas then
    hs.status = "Progressing"
    hs.message = string.format("Waiting for rollout to finish: %d out of %d new replicas have been updated...", updatedReplicas, specReplicas)
    return hs
  end
  if replicas > updatedReplicas then
    hs.status = "Progressing"
    hs.message = string.format("Waiting for rollout to finish: %d old replicas are pending termination...", replicas - updatedReplicas)
    return hs
  end
  if availableReplicas < updatedReplicas then
    hs.status = "Progressing"
    hs.message = string.format("Waiting for rollout to finish: %d of %d updated replicas are available...", availableReplicas, updatedReplicas)
    return hs
  end
else
  hs.status = "Progressing"
  hs.message = "Waiting for rollout to finish: observed deployment generation less than desired generation"
  return hs
end

hs.status = "Healthy"
return hs
