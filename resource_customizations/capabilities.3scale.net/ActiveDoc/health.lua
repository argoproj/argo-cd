local hs = {}

if obj.status ~= nil and obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Ready" and condition.status == "True" then
      hs.status = "Healthy"
      hs.message = "3scale ActiveDoc is ready"
      return hs
    elseif condition.type == "Invalid" and condition.status == "True" then
      hs.status = "Degraded"
      hs.message = condition.message or "3scale ActiveDoc configuration is invalid"
      return hs
    elseif condition.type == "Orphan" and condition.status == "True" then
      hs.status = "Degraded"
      hs.message = condition.message or "3scale ActiveDoc references non-existing resources"
      return hs
    elseif condition.type == "Failed" and condition.status == "True" then
      hs.status = "Degraded"
      hs.message = condition.message or "3scale ActiveDoc synchronization failed"
      return hs
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for 3scale ActiveDoc status..."
return hs
