-- KRO ResourceGraphDefinition: https://kro.run/docs/concepts/rgd/overview
local hs = {}

if obj.status == nil or obj.status.conditions == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for KRO to report status"
  return hs
end

local defined_conditions = {
  GraphRevisionsResolved = true,
  GraphAccepted = true,
  KindReady = true,
  ControllerReady = true,
  Ready = true,
}

local generation = obj.metadata and obj.metadata.generation

for _, condition in ipairs(obj.status.conditions) do
  if generation ~= nil and condition.observedGeneration ~= nil and condition.observedGeneration == generation then
    if condition.type == "Ready" and condition.status == "True" then
      hs.status = "Healthy"
      hs.message = "ResourceGraphDefinition is active and serving instances"
      return hs
    end

    if defined_conditions[condition.type] and condition.status == "False" then
      hs.status = "Degraded"
      hs.message = condition.message or (condition.type .. " is False")
      return hs
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for ResourceGraphDefinition to become ready"
return hs
