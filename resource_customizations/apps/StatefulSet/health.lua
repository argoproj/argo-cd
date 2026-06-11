local hs = {}

local function num(val)
  if val == nil then
    return 0
  end
  return val
end

local status = obj.status or {}
local observedGeneration = num(status.observedGeneration)
local generation = num(obj.metadata and obj.metadata.generation)

if observedGeneration == 0 or generation > observedGeneration then
  hs.status = "Progressing"
  hs.message = "Waiting for statefulset spec update to be observed..."
  return hs
end

local specReplicas = obj.spec and obj.spec.replicas
local readyReplicas = num(status.readyReplicas)
if specReplicas ~= nil and readyReplicas < specReplicas then
  hs.status = "Progressing"
  hs.message = string.format("Waiting for %d pods to be ready...", specReplicas - readyReplicas)
  return hs
end

local updateStrategy = obj.spec and obj.spec.updateStrategy
if updateStrategy ~= nil and updateStrategy.type == "RollingUpdate" and updateStrategy.rollingUpdate ~= nil then
  if specReplicas ~= nil and updateStrategy.rollingUpdate.partition ~= nil then
    local partition = updateStrategy.rollingUpdate.partition
    local updatedReplicas = num(status.updatedReplicas)
    if updatedReplicas < (specReplicas - partition) then
      hs.status = "Progressing"
      hs.message = string.format("Waiting for partitioned roll out to finish: %d out of %d new pods have been updated...", updatedReplicas, specReplicas - partition)
      return hs
    end
  end
  hs.status = "Healthy"
  hs.message = string.format("partitioned roll out complete: %d new pods have been updated...", num(status.updatedReplicas))
  return hs
end

if updateStrategy ~= nil and updateStrategy.type == "OnDelete" then
  hs.status = "Healthy"
  hs.message = string.format("statefulset has %d ready pods", readyReplicas)
  return hs
end

local updateRevision = status.updateRevision or ""
local currentRevision = status.currentRevision or ""
if updateRevision ~= currentRevision then
  hs.status = "Progressing"
  hs.message = string.format("waiting for statefulset rolling update to complete %d pods at revision %s...", num(status.updatedReplicas), updateRevision)
  return hs
end

hs.status = "Healthy"
hs.message = string.format("statefulset rolling update complete %d pods at revision %s...", num(status.currentReplicas), currentRevision)
return hs
