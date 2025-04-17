function checkStatus(condition)
  local hs = {}
  if condition.status == "False" then
    hs.status = "Degraded"
    hs.message = condition.message
    return hs
  end
  if condition.status == "True" then
    hs.status = "Healthy"
    hs.message = condition.message
    return hs
  end
end

local hs = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "Ready" and (obj.spec.refreshPolicy ~= "OnChange" or condition.lastTransitionTime >= obj.status.refreshTime) then
        return checkStatus(condition)
      end
    end
  end
end
hs.status = "Progressing"
hs.message = "Waiting for ExternalSecret"
return hs
