hs = {}
if obj.status == nil or obj.status.conditions == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for the status to be reported"
  return hs
end
if obj.status.observedGeneration ~= nil and obj.status.observedGeneration ~= obj.metadata.generation then
  hs.status = "Progressing"
  hs.message = "Waiting for the status to be updated"
  return hs  
end
for i, condition in ipairs(obj.status.conditions) do
  if condition.type == "Compliant" then
    hs.message = condition.message
    if condition.status == "True" then
      hs.status = "Healthy"
      return hs
    else
      hs.status = "Degraded"
      return hs
    end
  end
end
hs.status = "Progressing"
hs.message = "Waiting for the compliance condition"
return hs
