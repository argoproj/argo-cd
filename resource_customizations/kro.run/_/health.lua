-- KRO instance: https://kro.run/docs/concepts/instances/
local hs = {}

if obj.status == nil or obj.status.conditions == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for KRO to report status"
  return hs
end

local state = obj.status.state
local generation = obj.metadata and obj.metadata.generation

local defined_conditions = {
  InstanceManaged = true,
  GraphResolved = true,
  ResourcesReady = true,
  Ready = true,
}

for _, condition in ipairs(obj.status.conditions) do
  if generation ~= nil and condition.observedGeneration ~= nil and condition.observedGeneration == generation then
    if condition.type == "Ready" and condition.status == "True" then
      hs.status = "Healthy"
      hs.message = "Instance is active and running"
      return hs
    end

    if defined_conditions[condition.type] and condition.status == "False" then
      if state == "FAILED" or state == "ERROR" then
        hs.status = "Degraded"
        hs.message = condition.message or (condition.type .. " is False")
        return hs
      end
      if state == "IN_PROGRESS" then
        hs.status = "Progressing"
        hs.message = condition.message or (condition.type .. " is in progress")
        return hs
      end
    end
  end
end

-- No condition is False and Ready is not True yet
hs.status = "Progressing"
hs.message = "Waiting for instance to become ready"
return hs
