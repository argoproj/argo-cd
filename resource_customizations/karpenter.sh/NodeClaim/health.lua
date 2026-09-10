local hs = {}
if obj.metadata.generation ~= nil and obj.status ~= nil and obj.status.observedGeneration ~= nil then
  if obj.metadata.generation ~= obj.status.observedGeneration then
    hs.status = "Progressing"
    hs.message = "Waiting for NodeClaim spec to be reconciled"
    return hs
  end
end
if obj.status ~= nil and obj.status.conditions ~= nil then

  -- Disrupting takes priority: node is being terminated/consolidated/expired
  for i, condition in ipairs(obj.status.conditions) do
    if condition.type == "Disrupting" and condition.status == "True" then
      hs.status = "Suspended"
      hs.message = condition.message
      return hs
    end
  end

  for i, condition in ipairs(obj.status.conditions) do
    if condition.type == "Ready" then
      if condition.status == "True" then
        hs.status = "Healthy"
        hs.message = condition.message
        return hs
      elseif condition.status == "False" then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
      end
    end
  end

  -- Ready condition is Unknown or absent: report the furthest phase reached
  for i, condition in ipairs(obj.status.conditions) do
    if condition.type == "Initialized" and condition.status == "True" then
      hs.status = "Progressing"
      hs.message = "Node initialized, waiting for Ready"
      return hs
    end
  end
  for i, condition in ipairs(obj.status.conditions) do
    if condition.type == "Registered" and condition.status == "True" then
      hs.status = "Progressing"
      hs.message = "Node registered, waiting for initialization"
      return hs
    end
  end
  for i, condition in ipairs(obj.status.conditions) do
    if condition.type == "Launched" and condition.status == "True" then
      hs.status = "Progressing"
      hs.message = "Node launched, waiting for registration"
      return hs
    end
  end

end
hs.status = "Progressing"
hs.message = "Waiting for NodeClaim to be launched"
return hs
