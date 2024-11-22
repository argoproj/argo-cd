local hs={ status = "Progressing", message = "Waiting for initialization" }

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for _, condition in ipairs(obj.status.conditions) do
      if condition.type == "Deployed" and condition.status == "False" then
        hs.status = "Degraded"
        hs.message = condition.message or condition.reason
        return hs
      end
      if condition.type == "Deployed" and condition.status == "True" then
        hs.status = "Healthy"
        hs.message = condition.message or condition.reason
        return hs
      end
    end
  end
end


return hs
