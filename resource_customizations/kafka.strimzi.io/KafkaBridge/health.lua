local hs = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "NotReady" and condition.status == "True" then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
      end
      if condition.type == "Ready" and condition.status == "True" then
        hs.status = "Healthy"
        hs.message = ""
        return hs
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for KafkaBridge"
return hs
