hs = {}
if obj.status == nil or obj.status.compliant == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for the status to be reported"
  return hs
end
if obj.status.lastEvaluatedGeneration ~= obj.metadata.generation then
  hs.status = "Progressing"
  hs.message = "Waiting for the status to be updated"
  return hs  
end
if obj.status.compliant == "Compliant" then
  hs.status = "Healthy"
else
  hs.status = "Degraded"
end
if obj.status.compliancyDetails ~= nil then
  messages = {}
  for i, compliancy in ipairs(obj.status.compliancyDetails) do
    if compliancy.conditions ~= nil then
      for i, condition in ipairs(compliancy.conditions) do
        if condition.message ~= nil and condition.type ~= nil then
          table.insert(messages, condition.type .. " - " .. condition.message)
        end
      end
    end
  end
  hs.message = table.concat(messages, "; ")
  return hs
end
hs.status = "Progressing"
hs.message = "Waiting for compliance"
return hs 
