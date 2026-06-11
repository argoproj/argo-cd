local hs = {}

local function num(val)
  if val == nil then
    return 0
  end
  return val
end

local function getReplicaSetCondition(status, condType)
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

local generation = num(obj.metadata and obj.metadata.generation)
local status = obj.status or {}
local observedGeneration = num(status.observedGeneration)

if generation <= observedGeneration then
  local cond = getReplicaSetCondition(status, "ReplicaFailure")
  if cond ~= nil and cond.status == "True" then
    hs.status = "Degraded"
    hs.message = cond.message or ""
    return hs
  end
  local specReplicas = obj.spec and obj.spec.replicas
  local availableReplicas = num(status.availableReplicas)
  if specReplicas ~= nil and availableReplicas < specReplicas then
    hs.status = "Progressing"
    hs.message = string.format("Waiting for rollout to finish: %d out of %d new replicas are available...", availableReplicas, specReplicas)
    return hs
  end
else
  hs.status = "Progressing"
  hs.message = "Waiting for rollout to finish: observed replica set generation less than desired generation"
  return hs
end

hs.status = "Healthy"
return hs
