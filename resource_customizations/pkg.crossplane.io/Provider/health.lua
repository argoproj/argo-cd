local hs = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    local installed = false
    local healthy = false
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "Installed" then
        installed = condition.status == "True"
        installed_message = condition.reason
      elseif condition.type == "Healthy" then
        healthy = condition.status == "True"
        healthy_message = condition.reason
      end
    end
    if installed and healthy then
      hs.status = "Healthy"
    else
      hs.status = "Degraded"
    end
    hs.message = installed_message .. " " .. healthy_message
    return hs
  end
end

hs.status = "Progressing"
hs.message = "Waiting for provider to be installed"
return hs
