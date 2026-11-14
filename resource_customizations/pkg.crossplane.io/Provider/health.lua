local hs = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    local installed = false
    local healthy = false
    local installed_message
    local healthy_message
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "Installed" then
        installed = condition.status == "True"
        installed_message = condition.reason
      elseif condition.type == "Healthy" then
        healthy = condition.status == "True"
        healthy_message = condition.reason
      end
    end
    -- Crossplane can set one of these conditions without the other, for example
    -- only Installed=False when it cannot pull the package. With neither, such as
    -- a provider paused before its first reconcile, it is not installed yet.
    if installed_message ~= nil or healthy_message ~= nil then
      if installed and healthy then
        hs.status = "Healthy"
      else
        hs.status = "Degraded"
      end
      if installed_message ~= nil and healthy_message ~= nil then
        hs.message = installed_message .. " " .. healthy_message
      else
        hs.message = installed_message or healthy_message
      end
      return hs
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for provider to be installed"
return hs
