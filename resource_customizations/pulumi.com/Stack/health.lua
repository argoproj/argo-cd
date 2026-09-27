local hs = {}
hs.status = "Progressing"
hs.message = "Waiting for the stack to be reconciled"

if obj.status == nil or obj.status.observedGeneration ~= obj.metadata.generation then
  hs.message = "Waiting for the operator to observe the latest spec change"
  return hs
end

if obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Stalled" and condition.status == "True" then
      hs.status = "Degraded"
      hs.message = condition.message
      return hs
    end
    if condition.type == "Reconciling" and condition.status == "True" then
      hs.status = "Progressing"
      hs.message = condition.message
      return hs
    end
    if condition.type == "Ready" and condition.status == "True" then
      hs.status = "Healthy"
      hs.message = condition.message
      return hs
    end
  end
end

return hs
