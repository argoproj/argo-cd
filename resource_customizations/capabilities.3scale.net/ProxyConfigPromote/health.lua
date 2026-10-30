local hs = {}

if obj.status ~= nil and obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.status ~= "True" then goto continue end

    if condition.type == "Ready" then
      hs.status = "Healthy"
      hs.message = "3scale ProxyConfigPromote is ready"
      return hs
    elseif condition.type == "In-progress" then
      hs.status = "Progressing"
      hs.message = condition.message or "3scale ProxyConfigPromote is in progress"
      return hs
    elseif condition.type == "Failed" then
      hs.status = "Degraded"
      hs.message = condition.message or "3cale ProxyConfigPromote failed"
      return hs
    end

    ::continue::
  end
end

hs.status = "Progressing"
hs.message = "Waiting for 3scale ProxyConfigPromote status..."
return hs
