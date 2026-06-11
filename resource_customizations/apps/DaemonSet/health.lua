local hs = {}

local function num(val)
  if val == nil then
    return 0
  end
  return val
end

local generation = num(obj.metadata and obj.metadata.generation)
local status = obj.status or {}
local observedGeneration = num(status.observedGeneration)

if generation > observedGeneration then
  hs.status = "Progressing"
  hs.message = "Waiting for rollout to finish: observed daemon set generation less than desired generation"
  return hs
end

local updateStrategy = obj.spec and obj.spec.updateStrategy
if updateStrategy ~= nil and updateStrategy.type == "OnDelete" then
  hs.status = "Healthy"
  hs.message = string.format("daemon set %d out of %d new pods have been updated", num(status.updatedNumberScheduled), num(status.desiredNumberScheduled))
  return hs
end

local updatedNumberScheduled = num(status.updatedNumberScheduled)
local desiredNumberScheduled = num(status.desiredNumberScheduled)
if updatedNumberScheduled < desiredNumberScheduled then
  hs.status = "Progressing"
  hs.message = string.format('Waiting for daemon set %q rollout to finish: %d out of %d new pods have been updated...', obj.metadata.name, updatedNumberScheduled, desiredNumberScheduled)
  return hs
end

local numberAvailable = num(status.numberAvailable)
if numberAvailable < desiredNumberScheduled then
  hs.status = "Progressing"
  hs.message = string.format('Waiting for daemon set %q rollout to finish: %d of %d updated pods are available...', obj.metadata.name, numberAvailable, desiredNumberScheduled)
  return hs
end

hs.status = "Healthy"
return hs
