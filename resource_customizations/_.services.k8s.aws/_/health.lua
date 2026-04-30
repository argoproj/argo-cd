local hs = {}

if obj.status ~= nil and obj.status.conditions ~= nil then
  for i, condition in ipairs(obj.status.conditions) do

    if condition.type == "ACK.Terminal" and condition.status == "True" then
      hs.status = "Degraded"
      hs.message = condition.message
      return hs
    end
    
    if condition.type == "Ready" then
      if condition.status == "False" then
        hs.status = "Progressing"
        hs.message = condition.message
        return hs
      elseif condition.status == "True" then
        hs.status = "Healthy"
        hs.message = ""
        return hs
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for resource to be ready"
return hs