local hs = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    local healthy = false
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "Healthy" then
        healthy = condition.status == "True"
        healthy_message = condition.reason
      end
    end
    if healthy then
      hs.status = "Healthy"
    else
      hs.status = "Degraded"
    end
    hs.message = healthy_message
    return hs
  end
end

hs.status = "Progressing"
hs.message = "Waiting for ConfigurationRevision"
return hs
