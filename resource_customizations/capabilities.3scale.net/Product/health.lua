local hs = {}

if obj.status ~= nil and obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.status ~= "True" then goto continue end

    if condition.type == "Synced" then
      hs.status = "Healthy"
      hs.message = "3scale Product is synced"
      return hs
    elseif condition.type == "Invalid" then
      hs.status = "Degraded"
      hs.message = condition.message or "3scale Product configuration is invalid"
      return hs
    elseif condition.type == "Orphan" then
      hs.status = "Degraded"
      hs.message = condition.message or "3scale Product references non-existing resources"
      return hs
    elseif condition.type == "Failed" then
      hs.status = "Degraded"
      hs.message = condition.message or "3scale Product synchronization failed"
      return hs
    end

    ::continue::
  end
end

hs.status = "Progressing"
hs.message = "Waiting for 3scale Product status..."
return hs
