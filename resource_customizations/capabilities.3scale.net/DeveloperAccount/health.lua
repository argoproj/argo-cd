local hs = {}

if obj.status ~= nil and obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.status ~= "True" then goto continue end

    if condition.type == "Ready" then
      hs.status = "Healthy"
      hs.message = "3scale DeveloperAccount is ready"
      return hs
    elseif condition.type == "Invalid" then
      hs.status = "Degraded"
      hs.message = condition.message or "3scale DeveloperAccount configuration is invalid"
      return hs
    elseif condition.type == "Failed" then
      hs.status = "Degraded"
      hs.message = condition.message or "3scale DeveloperAccount synchronization failed"
      return hs
    elseif condition.type == "Waiting" then
      hs.status = "Suspended"
      hs.message = condition.message or "3scale DeveloperAccount is waiting for approval"
      return hs
    end

    ::continue::
  end
end

hs.status = "Progressing"
hs.message = "Waiting for 3scale DeveloperAccount status..."
return hs
