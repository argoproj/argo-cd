local hs = {}
if obj.status ~= nil and obj.status.conditions ~= nil then
  for i, condition in ipairs(obj.status.conditions) do
    if condition.type == "Ready" then
      if condition.status == "False" then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
      elseif condition.status == "True" then
        hs.status = "Healthy"
        hs.message = "EC2NodeClass is ready"
        return hs
      end
    end
  end
end
hs.status = "Progressing"
hs.message = "Waiting for EC2NodeClass to be ready"
return hs
