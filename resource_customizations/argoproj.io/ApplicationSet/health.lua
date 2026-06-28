local hs = {}

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in pairs(obj.status.conditions) do
      if condition.type == "ErrorOccurred" and condition.status == "True" then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
      end
    end
    for i, condition in pairs(obj.status.conditions) do
      if condition.type == "ResourcesUpToDate" and condition.status == "False" then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
      end
    end
    for i, condition in pairs(obj.status.conditions) do
      if condition.type == "RolloutProgressing" and condition.status == "True" then
        hs.status = "Progressing"
        hs.message = condition.message
        return hs
      end
    end
    for i, condition in pairs(obj.status.conditions) do
      if condition.type == "ResourcesUpToDate" and condition.status == "True" then
        hs.status = "Healthy"
        hs.message = condition.message
        return hs
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for the status to be reported"
return hs
