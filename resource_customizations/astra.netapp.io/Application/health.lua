hs = { status = "Progressing", message = "No status available" }
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for _, condition in ipairs(obj.status.conditions) do
      if condition.type == "Ready" and condition.status == "True" then
        hs.status = "Healthy"
        hs.message = "Astra Application Ready, protectionState: " .. obj.status.protectionState
        return hs
      elseif condition.type == "Ready" and condition.status == "False" then
        hs.status = "Degraded"
        hs.message = "Astra Application Degraded, message: " .. condition.message
        return hs
      end
    end
  end
end
return hs
